package engine

import (
	"crypto/sha1"
	"encoding/hex"
	"math"
	"sort"
	"strings"

	"github.com/muthuishere/citenexus/golang/gate"
)

// Phase A of the self-evolving brain: experiences → an evolving, itemized
// playbook. Everything here is deterministic and model-free — Reflect proposes
// candidate deltas from episode clusters, Regress gates them against what the
// brain already validated, Curate merges survivors into the playbook with
// content-dedup, supersession, and genealogy. The agent (skill layer) supplies
// prose; this core selects and structures. Files are git-friendly plain text so
// the repo's history is the audit trail.

// Delta is one proposed playbook change, distilled from evidence episodes.
type Delta struct {
	ID          string   `json:"id"`
	Rule        string   `json:"rule"`
	Valence     string   `json:"valence"`
	EvidenceIDs []string `json:"evidence_ids"`
	RootCause   string   `json:"root_cause"`
	Reward      float64  `json:"reward"` // mean reward across evidence
	Consistency float64  `json:"consistency"`
	Ts          float64  `json:"ts"`
	Status      string   `json:"status"` // proposed | applied | rejected
	Reason      string   `json:"reason,omitempty"`
}

// PlaybookEntry is one itemized, evolvable rule in the playbook.
type PlaybookEntry struct {
	ID           string   `json:"id"`
	Rule         string   `json:"rule"`
	Valence      string   `json:"valence"`
	EvidenceIDs  []string `json:"evidence_ids"`
	RootCause    string   `json:"root_cause,omitempty"`
	Reward       float64  `json:"reward"`
	SupportCount int      `json:"support_count"`
	SourceDeltas []string `json:"source_deltas,omitempty"`
	Supersedes   []string `json:"supersedes,omitempty"`
	SupersededBy string   `json:"superseded_by,omitempty"`
	Ts           float64  `json:"ts"`
	UpdatedTs    float64  `json:"updated_ts"`
}

// Active reports whether the entry is in force (not superseded).
func (e PlaybookEntry) Active() bool { return e.SupersededBy == "" }

// Playbook is the structured, itemized rule set an agent loads before acting.
// Never a growing blob: entries are deduped by content and superseded in place.
type Playbook struct {
	Entries []PlaybookEntry `json:"entries"`
}

// EvolvePolicy holds the acceptance thresholds for the learning loop. The
// evaluator lives OUTSIDE the loop: these come from config the producing run
// must not edit (evolve-policy.json, owned by the owner/CEO), with safe defaults.
type EvolvePolicy struct {
	MinConsistency       float64 `json:"min_consistency"`       // delta must be at least this internally consistent
	MinSharedTokens      int     `json:"min_shared_tokens"`     // clustering width for reflect
	ContradictionOverlap int     `json:"contradiction_overlap"` // shared tokens for a conviction to count as on-topic
	SupersessionOverlap  int     `json:"supersession_overlap"`  // shared tokens for a delta to supersede an entry
	MinImprovement       float64 `json:"min_improvement"`       // held-out gain a skill version needs to be promoted
}

// DefaultEvolvePolicy returns the standing defaults used when no policy file exists.
func DefaultEvolvePolicy() EvolvePolicy {
	return EvolvePolicy{MinConsistency: 0.6, MinSharedTokens: 2, ContradictionOverlap: 2, SupersessionOverlap: 2, MinImprovement: 0.05}
}

func contentKey(rule string) string {
	toks := gate.ContentTokens(strings.ToLower(rule))
	keys := make([]string, 0, len(toks))
	for t := range toks {
		keys = append(keys, t)
	}
	sort.Strings(keys)
	return strings.Join(keys, "\x00")
}

// DeltaID is a stable id from the rule content and its evidence.
func DeltaID(namespace, rule string, evidenceIDs []string) string {
	ids := append([]string(nil), evidenceIDs...)
	sort.Strings(ids)
	sum := sha1.Sum([]byte(namespace + "\x00" + contentKey(rule) + "\x00" + strings.Join(ids, "\x00")))
	return "pd_" + hex.EncodeToString(sum[:])[:16]
}

// EntryID is a stable id for a playbook entry, from rule content only (so the
// same lesson re-learned merges rather than duplicates).
func EntryID(namespace, rule string) string {
	sum := sha1.Sum([]byte(namespace + "\x00" + contentKey(rule)))
	return "pb_" + hex.EncodeToString(sum[:])[:16]
}

// Reflect distills episodes recorded since sinceTs (successes AND failures)
// into candidate playbook deltas. Read-only: it proposes, never applies.
func Reflect(namespace string, episodes []Episode, sinceTs float64, policy EvolvePolicy, now float64) []Delta {
	var recent []Episode
	for _, e := range episodes {
		if e.Active() && e.Outcome != nil && e.Ts >= sinceTs {
			recent = append(recent, e)
		}
	}
	clusters := ClusterByTokens(recent, policy.MinSharedTokens)
	var out []Delta
	for _, cluster := range clusters {
		v := AssessCluster(cluster, 1, 0)
		if v.Valence == "neutral" || len(v.SupportingIDs) == 0 {
			continue
		}
		var supporting []Episode
		for _, e := range cluster {
			for _, id := range v.SupportingIDs {
				if e.ID == id {
					supporting = append(supporting, e)
				}
			}
		}
		sort.SliceStable(supporting, func(i, j int) bool {
			return math.Abs(supporting[i].Outcome.RPE()) > math.Abs(supporting[j].Outcome.RPE())
		})
		anchor := supporting[0]
		rule := "DO: " + anchor.Text
		if v.Valence == "negative" {
			rule = "AVOID: " + anchor.Text
		}
		sum := 0.0
		for _, e := range supporting {
			sum += e.Outcome.Reward
		}
		rootCause := anchor.Outcome.Label
		if rootCause == "" {
			rootCause = v.Valence + " outcome, highest-surprise evidence: " + anchor.ID
		}
		out = append(out, Delta{
			ID: DeltaID(namespace, rule, v.SupportingIDs), Rule: rule, Valence: v.Valence,
			EvidenceIDs: v.SupportingIDs, RootCause: rootCause,
			Reward: sum / float64(len(supporting)), Consistency: v.Consistency,
			Ts: now, Status: "proposed",
		})
	}
	return out
}

// RegressResult is the gate's ruling on one proposed delta.
type RegressResult struct {
	DeltaID string `json:"delta_id"`
	Passed  bool   `json:"passed"`
	Reason  string `json:"reason"`
}

// Regress is the held-in regression gate: a proposed delta is rejected if it is
// too inconsistent, or if it contradicts a validated conviction (opposite
// valence on the same topic) — i.e. it would overwrite something that already
// proved out. Rejections are for logging, never silent application.
func Regress(delta Delta, convictions []Conviction, policy EvolvePolicy) RegressResult {
	if delta.Consistency < policy.MinConsistency {
		return RegressResult{DeltaID: delta.ID, Passed: false,
			Reason: "consistency below policy threshold"}
	}
	dToks := gate.ContentTokens(delta.Rule)
	for _, cv := range convictions {
		if !cv.Active() || cv.Valence == delta.Valence || cv.Valence == "neutral" {
			continue
		}
		if sharedCount(dToks, gate.ContentTokens(cv.Statement)) >= policy.ContradictionOverlap {
			return RegressResult{DeltaID: delta.ID, Passed: false,
				Reason: "contradicts validated conviction " + cv.ID + ": " + cv.Statement}
		}
	}
	return RegressResult{DeltaID: delta.ID, Passed: true, Reason: "no contradiction with validated convictions"}
}

// CurateReport summarizes what a curate pass did.
type CurateReport struct {
	Merged     int `json:"merged"`     // deltas folded into an existing entry (content dedup)
	Added      int `json:"added"`      // brand-new entries
	Superseded int `json:"superseded"` // stale entries retired by an opposing rule
	Total      int `json:"total"`      // active entries after the pass
}

// Curate merges accepted deltas into the playbook: dedup by CONTENT, supersede
// stale opposing rules, keep genealogy. Mutates pb in place; the caller decides
// whether to persist (--apply) and commits the folder for the audit trail.
func Curate(namespace string, pb *Playbook, deltas []Delta, policy EvolvePolicy, now float64) CurateReport {
	rep := CurateReport{}
	for _, d := range deltas {
		id := EntryID(namespace, d.Rule)
		merged := false
		for i := range pb.Entries {
			e := &pb.Entries[i]
			if e.ID == id && e.Active() {
				e.EvidenceIDs = unionIDs(e.EvidenceIDs, d.EvidenceIDs)
				e.SupportCount = len(e.EvidenceIDs)
				e.Reward = (e.Reward + d.Reward) / 2
				e.SourceDeltas = unionIDs(e.SourceDeltas, []string{d.ID})
				e.UpdatedTs = now
				merged = true
				rep.Merged++
				break
			}
		}
		if merged {
			continue
		}
		entry := PlaybookEntry{
			ID: id, Rule: d.Rule, Valence: d.Valence, EvidenceIDs: d.EvidenceIDs,
			RootCause: d.RootCause, Reward: d.Reward, SupportCount: len(d.EvidenceIDs),
			SourceDeltas: []string{d.ID}, Ts: now, UpdatedTs: now,
		}
		// An opposing rule on the same topic is stale — retire it, keep lineage.
		dToks := gate.ContentTokens(d.Rule)
		for i := range pb.Entries {
			e := &pb.Entries[i]
			if !e.Active() || e.Valence == d.Valence {
				continue
			}
			if sharedCount(dToks, gate.ContentTokens(e.Rule)) >= policy.SupersessionOverlap {
				e.SupersededBy = entry.ID
				entry.Supersedes = append(entry.Supersedes, e.ID)
				rep.Superseded++
			}
		}
		pb.Entries = append(pb.Entries, entry)
		rep.Added++
	}
	for _, e := range pb.Entries {
		if e.Active() {
			rep.Total++
		}
	}
	return rep
}

// TopicEntries filters active entries whose rule shares a content token with topic.
func (pb Playbook) TopicEntries(topic string) []PlaybookEntry {
	tToks := gate.ContentTokens(topic)
	var out []PlaybookEntry
	for _, e := range pb.Entries {
		if e.Active() && (topic == "" || sharedCount(tToks, gate.ContentTokens(e.Rule)) >= 1) {
			out = append(out, e)
		}
	}
	return out
}

func unionIDs(a, b []string) []string {
	seen := map[string]bool{}
	out := append([]string(nil), a...)
	for _, id := range a {
		seen[id] = true
	}
	for _, id := range b {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// Episodes exposes the store's episodes for read-only passes (reflect).
func (b *Brain) Episodes() []Episode { return b.store.Episodes() }

// Namespace exposes the brain's namespace for stable delta/entry ids.
func (b *Brain) Namespace() string { return b.namespace }

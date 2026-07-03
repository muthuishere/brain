// Package brain is the Go port of the CiteNexus Brain's deterministic core:
// episodic memory with salience-weighted, cite-or-abstain recall, and the
// constraint shield (the "profitable but ruinous" veto). It mirrors the Python
// reference (src/citenexus/brain) byte-for-byte on the parts that must agree
// across languages — the recall gate and the shield are pure, model-free logic.
//
// The LLM and embedder are injected interfaces, so the whole thing runs on the
// deterministic fakes with no network. See docs/BRAIN.md.
package engine

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
)

// Embedder turns text into a vector (a real endpoint, or fakes.FakeEmbedding).
type Embedder interface{ Embed(text string) []float64 }

// LLM synthesizes a grounded answer from a passage (or fakes.FakeLLM).
type LLM interface {
	Answer(question, passage string) string
}

// Outcome is signed feedback about an episode along some dimension.
type Outcome struct {
	Reward    float64 `json:"reward"`
	Baseline  float64 `json:"baseline,omitempty"`
	Dimension string  `json:"dimension,omitempty"`
	Label     string  `json:"label,omitempty"`
	Note      string  `json:"note,omitempty"`
}

// RPE is the reward-prediction error — the surprise that drives salience.
func (o Outcome) RPE() float64 { return o.Reward - o.Baseline }

// Valence is the general sign of the feedback.
func (o Outcome) Valence() string {
	switch {
	case o.Reward > o.Baseline:
		return "positive"
	case o.Reward < o.Baseline:
		return "negative"
	default:
		return "neutral"
	}
}

// Episode is one recorded experience, kept verbatim and citeable.
type Episode struct {
	ID            string    `json:"id"`
	Text          string    `json:"text"`
	Cue           string    `json:"cue"`
	Version       int       `json:"version"`
	Ts            float64   `json:"ts"`
	Embedding     []float64 `json:"-"` // recomputed on read (deterministic) — keeps files git-clean
	Outcome       *Outcome  `json:"outcome,omitempty"`
	PriorOutcomes []Outcome `json:"prior_outcomes,omitempty"`
	SupersededBy  string    `json:"superseded_by,omitempty"`
}

// Active reports whether this episode is live (not soft-decayed/superseded).
func (e Episode) Active() bool { return e.SupersededBy == "" }

// Reappraised reports whether the episode's verdict has been revised at least once.
func (e Episode) Reappraised() bool { return len(e.PriorOutcomes) > 0 }

// BaseSalience is |RPE| with losses up-weighted, scaled into a magnitude.
func BaseSalience(o *Outcome, lossAversion, scale float64) float64 {
	if o == nil {
		return 0
	}
	m := math.Abs(o.RPE())
	if o.RPE() < 0 {
		m *= lossAversion
	}
	if scale > 0 {
		return m / scale
	}
	return m
}

// Importance maps raw salience into [0,1).
func Importance(salience float64) float64 { return 1.0 - math.Exp(-math.Max(salience, 0)) }

// Recalled is an episode the brain drew on, with its recall score.
type Recalled struct {
	Episode   Episode
	Score     float64
	Relevance float64
}

// Recall is the result of Ask — a grounded answer or an honest abstention.
type Recall struct {
	Question string
	Answer   string
	Grounded bool
	Conflict bool
	Reason   string
	Episodes []Recalled
}

// Brain records experiences and recalls them, grounded or abstaining.
type Brain struct {
	embedder     Embedder
	llm          LLM
	namespace    string
	store        Store
	lossAversion float64
	scale        float64
	shield       Shield
	now          func() float64
}

// New builds a brain backed by an in-memory store. Pass fakes.FakeEmbedding{} and
// fakes.FakeLLM{} to run offline. Use NewWithStore for a persistent (file) store.
func New(embedder Embedder, llm LLM, namespace string, constraints []Constraint, highReward float64) *Brain {
	return NewWithStore(embedder, llm, namespace, constraints, highReward, NewInMemoryStore(), nil)
}

// NewWithStore builds a brain over an explicit store (e.g. FileStore) and clock.
func NewWithStore(embedder Embedder, llm LLM, namespace string, constraints []Constraint, highReward float64, store Store, now func() float64) *Brain {
	if now == nil {
		now = func() float64 { return 0 }
	}
	return &Brain{
		embedder:     embedder,
		llm:          llm,
		namespace:    namespace,
		store:        store,
		lossAversion: 1.5,
		scale:        1.0,
		shield:       Shield{constraints: constraints, highReward: highReward},
		now:          now,
	}
}

func episodeID(namespace, text string, version int) string {
	sum := sha1.Sum([]byte(fmt.Sprintf("%s\x00%d\x00%s", namespace, version, text)))
	return "ep_" + hex.EncodeToString(sum[:])[:16]
}

// Record stores a raw experience. Cheap and synchronous — no model call.
func (b *Brain) Record(text string, outcome *Outcome) Episode {
	text = strings.TrimSpace(text)
	version := b.store.NextVersion()
	ep := Episode{
		ID:        episodeID(b.namespace, text, version),
		Text:      text,
		Cue:       text,
		Version:   version,
		Ts:        b.now(),
		Embedding: b.embedder.Embed(text),
		Outcome:   outcome,
	}
	b.store.AddEpisode(ep)
	return ep
}

// Reappraise attaches or updates the outcome of an earlier episode by id. The
// prior verdict is preserved (the audit trail humans lack); a flip is salient.
func (b *Brain) Reappraise(id string, outcome Outcome) bool {
	for _, ep := range b.store.Episodes() {
		if ep.ID == id {
			if ep.Outcome != nil {
				ep.PriorOutcomes = append(ep.PriorOutcomes, *ep.Outcome)
			}
			ep.Outcome = &outcome
			b.store.ReplaceEpisode(ep)
			return true
		}
	}
	return false
}

// Ask recalls a grounded answer, or abstains when memory is too thin.
func (b *Brain) Ask(question string, k int) Recall {
	ranked := b.recall(question, k)
	if len(ranked) == 0 {
		return Recall{Question: question, Answer: b.cantAnswer(), Grounded: false,
			Reason: "no sufficiently relevant experience recorded"}
	}
	conflict := valenceConflict(ranked)
	top := ranked[0].Episode
	if b.llm == nil {
		return Recall{Question: question, Answer: top.Text, Grounded: true,
			Conflict: conflict, Episodes: ranked}
	}
	draft := b.llm.Answer(question, top.Text)
	if !IsSupported(draft, top.Text) {
		return Recall{Question: question, Answer: b.cantAnswer(), Grounded: false,
			Reason: "recalled experience did not support a grounded answer"}
	}
	return Recall{Question: question, Answer: draft, Grounded: true,
		Conflict: conflict, Episodes: ranked}
}

func (b *Brain) recall(question string, k int) []Recalled {
	q := b.embedder.Embed(question)
	episodes := b.store.Episodes()
	out := make([]Recalled, 0, len(episodes))
	for _, ep := range episodes {
		if !ep.Active() {
			continue
		}
		emb := ep.Embedding
		if len(emb) == 0 { // reloaded from disk — recompute deterministically
			emb = b.embedder.Embed(ep.Cue)
		}
		rel := Cosine(q, emb)
		if !HasRelevanceOverlap(question, ep.Cue) && !HasRelevanceOverlap(question, ep.Text) {
			continue
		}
		score := rel + Importance(BaseSalience(ep.Outcome, b.lossAversion, b.scale))
		out = append(out, Recalled{Episode: ep, Score: score, Relevance: rel})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Episode.Version > out[j].Episode.Version
	})
	if len(out) > k {
		out = out[:k]
	}
	return out
}

// Check weighs a proposed decision against the standing constraints.
func (b *Brain) Check(ctx DecisionContext, objectiveReward float64, fallback string) Verdict {
	return b.shield.Evaluate(ctx, objectiveReward, fallback)
}

func (b *Brain) cantAnswer() string {
	return "I don't have enough recorded experience to answer that."
}

// SetObjective streams in a new foreground objective; the previous one retires to
// history. Convictions from the retired frame go dormant (kept, revivable); those
// from a returning frame revive.
func (b *Brain) SetObjective(text, agentID string) Objective {
	text = strings.TrimSpace(text)
	var retiring string
	if cur, ok := b.store.CurrentObjective(); ok {
		retiring = cur.Text
	}
	obj := Objective{Text: text, Ts: b.now(), Version: b.store.NextObjectiveVersion(), AgentID: agentID}
	b.store.PushObjective(obj)
	for _, cv := range b.store.Convictions() {
		if cv.GoalFrame == retiring && retiring != "" && cv.GoalFrame != text && !cv.Dormant {
			cv.Dormant = true
			b.store.PutConviction(cv)
		} else if cv.GoalFrame == text && cv.Dormant {
			cv.Dormant = false
			b.store.PutConviction(cv)
		}
	}
	return obj
}

// Objective returns the current foreground objective, if any.
func (b *Brain) Objective() (Objective, bool) { return b.store.CurrentObjective() }

// Convictions returns the brain's active point of view, most-confident first.
func (b *Brain) Convictions(includeDormant bool) []Conviction {
	var out []Conviction
	for _, cv := range b.store.Convictions() {
		if includeDormant || cv.Active() {
			out = append(out, cv)
		}
	}
	return out
}

// ConsolidationReport summarizes what a consolidation pass changed.
type ConsolidationReport struct {
	ConvictionsFormed int `json:"convictions_formed"`
	ConflictsSurfaced int `json:"conflicts_surfaced"`
	EpisodesDecayed   int `json:"episodes_decayed"`
	TotalConvictions  int `json:"total_convictions"`
}

// Consolidate is the "sleep" pass: promote repeated, consistent signal into
// validated convictions. Deterministic throughout (extractive phrasing when no LLM).
func (b *Brain) Consolidate(minSupport int, minConsistency float64, minSharedTokens int, forget bool, forgetAfterHours float64) ConsolidationReport {
	clusters := ClusterByTokens(b.store.Episodes(), minSharedTokens)
	frame := ""
	if cur, ok := b.store.CurrentObjective(); ok {
		frame = cur.Text
	}
	formed, conflicts := 0, 0
	supported := map[string]bool{}
	existing := map[string]bool{}
	for _, cv := range b.store.Convictions() {
		existing[cv.ID] = true
	}
	for _, cluster := range clusters {
		v := AssessCluster(cluster, minSupport, minConsistency)
		if v.Conflicted {
			conflicts++
		}
		if !v.IsConviction {
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
		statement := b.distill(supporting)
		id := ConvictionID(b.namespace, statement, v.SupportingIDs)
		if !existing[id] {
			formed++
		}
		b.store.PutConviction(Conviction{
			ID: id, Namespace: b.namespace, Statement: statement, Valence: v.Valence,
			SupportingIDs: v.SupportingIDs, SupportCount: v.SupportCount, Consistency: v.Consistency,
			Confidence: ConfidenceOf(v.SupportCount, v.Consistency), GoalFrame: frame, Ts: b.now(),
		})
		for _, id := range v.SupportingIDs {
			supported[id] = true
		}
	}
	decayed := 0
	if forget {
		decayed = b.forgetStale(supported, forgetAfterHours)
	}
	return ConsolidationReport{
		ConvictionsFormed: formed, ConflictsSurfaced: conflicts, EpisodesDecayed: decayed,
		TotalConvictions: len(b.Convictions(false)),
	}
}

func (b *Brain) distill(supporting []Episode) string {
	sort.SliceStable(supporting, func(i, j int) bool {
		return BaseSalience(supporting[i].Outcome, b.lossAversion, b.scale) >
			BaseSalience(supporting[j].Outcome, b.lossAversion, b.scale)
	})
	anchor := supporting[0].Text
	if b.llm == nil {
		return anchor
	}
	seen := map[string]bool{}
	var distinct []string
	for _, e := range supporting {
		if !seen[e.Text] {
			seen[e.Text] = true
			distinct = append(distinct, e.Text)
		}
	}
	union := strings.Join(distinct, " ")
	draft := b.llm.Answer("What is the recurring lesson here?", union)
	if IsSupported(draft, union) {
		return draft
	}
	return anchor
}

func (b *Brain) forgetStale(supported map[string]bool, forgetAfterHours float64) int {
	decayed := 0
	for _, ep := range b.store.Episodes() {
		if !ep.Active() || supported[ep.ID] {
			continue
		}
		ageHours := (b.now() - ep.Ts) / 3600.0
		if ageHours < 0 {
			ageHours = 0
		}
		if ageHours >= forgetAfterHours && BaseSalience(ep.Outcome, b.lossAversion, b.scale) <= 0 {
			ep.SupersededBy = "decayed"
			b.store.ReplaceEpisode(ep)
			decayed++
		}
	}
	return decayed
}

func valenceConflict(ranked []Recalled) bool {
	pos, neg := false, false
	for _, r := range ranked {
		if r.Episode.Outcome == nil {
			continue
		}
		switch r.Episode.Outcome.Valence() {
		case "positive":
			pos = true
		case "negative":
			neg = true
		}
	}
	return pos && neg
}

package engine

import (
	"strings"
	"testing"
)

func ep(id, text string, reward, ts float64) Episode {
	return Episode{ID: id, Text: text, Cue: text, Ts: ts, Outcome: &Outcome{Reward: reward}}
}

func TestReflectProposesDeltasFromSuccessAndFailure(t *testing.T) {
	eps := []Episode{
		ep("e1", "overtrading in chop bled the account", -1, 10),
		ep("e2", "overtrading in chop again bled the account", -0.8, 20),
		ep("e3", "waiting for the breakout confirmation paid off", 1, 30),
		ep("e4", "old stale episode before window", -1, 1),
	}
	deltas := Reflect("ns", eps, 5, DefaultEvolvePolicy(), 100)
	if len(deltas) != 2 {
		t.Fatalf("want 2 deltas, got %d: %+v", len(deltas), deltas)
	}
	var neg, pos *Delta
	for i := range deltas {
		if deltas[i].Valence == "negative" {
			neg = &deltas[i]
		} else {
			pos = &deltas[i]
		}
	}
	if neg == nil || pos == nil {
		t.Fatalf("want one negative and one positive delta: %+v", deltas)
	}
	if !strings.HasPrefix(neg.Rule, "AVOID: ") || !strings.HasPrefix(pos.Rule, "DO: ") {
		t.Fatalf("rule prefixes wrong: %q / %q", neg.Rule, pos.Rule)
	}
	if len(neg.EvidenceIDs) != 2 {
		t.Fatalf("negative delta should cite both episodes, got %v", neg.EvidenceIDs)
	}
	// Deterministic: same input → same ids.
	again := Reflect("ns", eps, 5, DefaultEvolvePolicy(), 100)
	if again[0].ID != deltas[0].ID || again[1].ID != deltas[1].ID {
		t.Fatal("reflect is not deterministic")
	}
}

func TestRegressRejectsContradictionAndLowConsistency(t *testing.T) {
	policy := DefaultEvolvePolicy()
	cv := Conviction{ID: "cv1", Statement: "breakout confirmation entries are profitable", Valence: "positive"}

	contradicting := Delta{ID: "d1", Rule: "AVOID: breakout confirmation entries", Valence: "negative", Consistency: 1}
	if res := Regress(contradicting, []Conviction{cv}, policy); res.Passed {
		t.Fatalf("contradicting delta should be rejected: %+v", res)
	}
	weak := Delta{ID: "d2", Rule: "AVOID: something unrelated entirely", Valence: "negative", Consistency: 0.4}
	if res := Regress(weak, nil, policy); res.Passed {
		t.Fatal("low-consistency delta should be rejected")
	}
	fine := Delta{ID: "d3", Rule: "AVOID: revenge trades after a stop-out", Valence: "negative", Consistency: 1}
	if res := Regress(fine, []Conviction{cv}, policy); !res.Passed {
		t.Fatalf("clean delta should pass: %+v", res)
	}
	// Dormant convictions don't gate.
	dormant := cv
	dormant.Dormant = true
	if res := Regress(contradicting, []Conviction{dormant}, policy); !res.Passed {
		t.Fatal("dormant conviction should not gate")
	}
}

func TestCurateDedupSupersedeGenealogy(t *testing.T) {
	policy := DefaultEvolvePolicy()
	pb := Playbook{}
	d1 := Delta{ID: "pd1", Rule: "AVOID: overtrading in chop", Valence: "negative",
		EvidenceIDs: []string{"e1"}, Reward: -1, Consistency: 1}
	rep := Curate("ns", &pb, []Delta{d1}, policy, 100)
	if rep.Added != 1 || rep.Total != 1 {
		t.Fatalf("first curate: %+v", rep)
	}
	// Same content again (different evidence) → merged, not duplicated.
	d2 := d1
	d2.ID = "pd2"
	d2.EvidenceIDs = []string{"e2"}
	rep = Curate("ns", &pb, []Delta{d2}, policy, 200)
	if rep.Merged != 1 || rep.Added != 0 || len(pb.Entries) != 1 {
		t.Fatalf("dedup failed: %+v entries=%d", rep, len(pb.Entries))
	}
	e := pb.Entries[0]
	if e.SupportCount != 2 || len(e.SourceDeltas) != 2 {
		t.Fatalf("merge lost lineage: %+v", e)
	}
	// Opposing rule on the same topic supersedes with genealogy.
	d3 := Delta{ID: "pd3", Rule: "DO: overtrading in chop is fine with tight stops", Valence: "positive",
		EvidenceIDs: []string{"e3"}, Reward: 1, Consistency: 1}
	rep = Curate("ns", &pb, []Delta{d3}, policy, 300)
	if rep.Superseded != 1 || rep.Total != 1 {
		t.Fatalf("supersession failed: %+v", rep)
	}
	old, neu := pb.Entries[0], pb.Entries[1]
	if old.Active() || old.SupersededBy != neu.ID {
		t.Fatalf("old entry not retired properly: %+v", old)
	}
	if len(neu.Supersedes) != 1 || neu.Supersedes[0] != old.ID {
		t.Fatalf("new entry missing genealogy: %+v", neu)
	}
	// Topic filter only returns active on-topic rules.
	got := pb.TopicEntries("overtrading")
	if len(got) != 1 || got[0].ID != neu.ID {
		t.Fatalf("topic filter: %+v", got)
	}
}

package engine

import (
	"math"
	"testing"
)

// TestClusterByTokensChainMergesViaUnionFind builds a chain A-B-C where A and B
// share a content token, B and C share a (different) content token, but A and C
// share none directly. Union-find should still merge all three into one cluster.
// A fourth, disjoint episode stays its own singleton. Episodes with no Outcome
// or that are inactive (superseded) must never appear in any returned cluster.
func TestClusterByTokensChainMergesViaUnionFind(t *testing.T) {
	pos := &Outcome{Reward: 1, Baseline: 0}

	epA := Episode{ID: "ep_a", Cue: "alpha bravo charlie", Text: "alpha bravo charlie", Outcome: pos}
	epB := Episode{ID: "ep_b", Cue: "bravo delta echo", Text: "bravo delta echo", Outcome: pos}
	epC := Episode{ID: "ep_c", Cue: "echo foxtrot golf", Text: "echo foxtrot golf", Outcome: pos}
	epD := Episode{ID: "ep_d", Cue: "zulu yankee xray", Text: "zulu yankee xray", Outcome: pos}
	// Excluded: no outcome at all.
	epE := Episode{ID: "ep_e", Cue: "alpha bravo charlie", Text: "alpha bravo charlie", Outcome: nil}
	// Excluded: inactive (superseded), even though it has an outcome and shares tokens with A.
	epF := Episode{ID: "ep_f", Cue: "alpha bravo charlie", Text: "alpha bravo charlie", Outcome: pos, SupersededBy: "ep_z"}

	clusters := ClusterByTokens([]Episode{epA, epB, epC, epD, epE, epF}, 1)

	find := func(id string) []Episode {
		for _, c := range clusters {
			for _, e := range c {
				if e.ID == id {
					return c
				}
			}
		}
		return nil
	}

	abc := find("ep_a")
	if len(abc) != 3 {
		t.Fatalf("expected chain A-B-C to merge into one 3-episode cluster, got %d: %+v", len(abc), abc)
	}
	seen := map[string]bool{}
	for _, e := range abc {
		seen[e.ID] = true
	}
	if !seen["ep_b"] || !seen["ep_c"] {
		t.Fatalf("expected cluster to contain B and C transitively via union-find, got %+v", abc)
	}

	d := find("ep_d")
	if len(d) != 1 {
		t.Fatalf("expected disjoint episode D to remain its own singleton cluster, got %d", len(d))
	}

	if find("ep_e") != nil {
		t.Fatalf("episode with a nil Outcome must be excluded from clustering entirely")
	}
	if find("ep_f") != nil {
		t.Fatalf("inactive (superseded) episode must be excluded from clustering entirely")
	}

	if len(clusters) != 2 {
		t.Fatalf("expected exactly 2 clusters ({A,B,C} and {D}), got %d: %+v", len(clusters), clusters)
	}
}

// TestAssessClusterIsConvictionWhenSupportAndConsistencyClear checks the
// straightforward case: enough positive-outcome episodes, unanimous valence,
// clearing both minSupport and minConsistency.
func TestAssessClusterIsConvictionWhenSupportAndConsistencyClear(t *testing.T) {
	pos := Outcome{Reward: 1, Baseline: 0}
	cluster := []Episode{
		{ID: "ep_1", Outcome: &pos},
		{ID: "ep_2", Outcome: &pos},
		{ID: "ep_3", Outcome: &pos},
	}

	v := AssessCluster(cluster, 2, 0.7)

	if !v.IsConviction {
		t.Fatalf("expected IsConviction=true, got %+v", v)
	}
	if v.Conflicted {
		t.Fatalf("expected Conflicted=false, got true: %+v", v)
	}
	if v.Valence != "positive" {
		t.Fatalf("expected Valence=positive, got %q", v.Valence)
	}
	if v.SupportCount != 3 {
		t.Fatalf("expected SupportCount=3, got %d", v.SupportCount)
	}
	if math.Abs(v.Consistency-1.0) > 1e-9 {
		t.Fatalf("expected Consistency=1.0, got %v", v.Consistency)
	}
	wantIDs := map[string]bool{"ep_1": true, "ep_2": true, "ep_3": true}
	if len(v.SupportingIDs) != 3 {
		t.Fatalf("expected 3 supporting ids, got %v", v.SupportingIDs)
	}
	for _, id := range v.SupportingIDs {
		if !wantIDs[id] {
			t.Fatalf("unexpected supporting id %q in %v", id, v.SupportingIDs)
		}
	}
}

// TestAssessClusterConflictedWhenNeitherSideClearsConsistency: 3 positive vs 2
// negative out of 5 gives consistency 3/5=0.6, which clears neither side of a
// 0.7 minConsistency bar. That must surface as Conflicted, not silently pick
// the majority side as a conviction.
func TestAssessClusterConflictedWhenNeitherSideClearsConsistency(t *testing.T) {
	pos := Outcome{Reward: 1, Baseline: 0}
	neg := Outcome{Reward: 0, Baseline: 1}
	cluster := []Episode{
		{ID: "ep_1", Outcome: &pos},
		{ID: "ep_2", Outcome: &pos},
		{ID: "ep_3", Outcome: &pos},
		{ID: "ep_4", Outcome: &neg},
		{ID: "ep_5", Outcome: &neg},
	}

	v := AssessCluster(cluster, 3, 0.7)

	if math.Abs(v.Consistency-0.6) > 1e-9 {
		t.Fatalf("expected consistency 3/5=0.6, got %v", v.Consistency)
	}
	if !v.Conflicted {
		t.Fatalf("expected Conflicted=true (total>=minSupport, consistency<minConsistency), got %+v", v)
	}
	if v.IsConviction {
		t.Fatalf("expected IsConviction=false when consistency does not clear the bar, got true: %+v", v)
	}
}

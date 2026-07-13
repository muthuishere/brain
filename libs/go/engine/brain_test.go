package engine

import (
	"testing"

	"github.com/muthuishere/citenexus/golang/fakes"
)

// TestAskAbstainsOnEmptyStore: with nothing recorded, Ask must abstain with a
// reason rather than fabricate an answer.
func TestAskAbstainsOnEmptyStore(t *testing.T) {
	b := New(fakes.FakeEmbedding{}, nil, "ns", nil, 10)

	r := b.Ask("what happened last week", 3)

	if r.Grounded {
		t.Fatalf("expected Grounded=false on an empty store, got true (answer=%q)", r.Answer)
	}
	if r.Reason == "" {
		t.Fatalf("expected a non-empty abstention Reason")
	}
}

// TestAskAbstainsWhenNoRelevanceOverlap: a populated store still abstains if
// the question shares no content token with any recorded episode's Cue/Text.
func TestAskAbstainsWhenNoRelevanceOverlap(t *testing.T) {
	b := New(fakes.FakeEmbedding{}, nil, "ns", nil, 10)
	b.Record("cats are wonderful household pets", &Outcome{Reward: 1, Baseline: 0})

	r := b.Ask("what is the weather forecast tomorrow", 3)

	if r.Grounded {
		t.Fatalf("expected Grounded=false when question has no relevance overlap, got true (answer=%q)", r.Answer)
	}
	if r.Reason == "" {
		t.Fatalf("expected a non-empty abstention Reason")
	}
}

// TestAskGroundedAnswerVerbatimWithoutLLM: a relevant episode exists and no
// LLM is configured, so Ask must return the episode's Text verbatim.
func TestAskGroundedAnswerVerbatimWithoutLLM(t *testing.T) {
	b := New(fakes.FakeEmbedding{}, nil, "ns", nil, 10)
	ep := b.Record("deploying on friday afternoons causes weekend incidents", &Outcome{Reward: 1, Baseline: 0})

	r := b.Ask("what happens when deploying on friday afternoons", 3)

	if !r.Grounded {
		t.Fatalf("expected Grounded=true, got false (reason=%q)", r.Reason)
	}
	if r.Answer != ep.Text {
		t.Fatalf("expected verbatim episode text with nil llm, got %q want %q", r.Answer, ep.Text)
	}
}

// TestAskGroundedAnswerWithLLM exercises the b.llm != nil path. FakeLLM echoes
// the cited passage verbatim, so gate.IsSupported passes honestly (not bypassed).
func TestAskGroundedAnswerWithLLM(t *testing.T) {
	b := New(fakes.FakeEmbedding{}, fakes.FakeLLM{}, "ns", nil, 10)
	ep := b.Record("deploying on friday afternoons causes weekend incidents", &Outcome{Reward: 1, Baseline: 0})

	r := b.Ask("what happens when deploying on friday afternoons", 3)

	if !r.Grounded {
		t.Fatalf("expected Grounded=true, got false (reason=%q)", r.Reason)
	}
	if r.Answer != ep.Text {
		t.Fatalf("expected FakeLLM-echoed answer to equal the cited passage, got %q want %q", r.Answer, ep.Text)
	}
}

// TestAskValenceConflictWithoutSuppressingGroundedAnswer: two episodes overlap
// the question and each other, one positive-outcome and one negative-outcome.
// Both get recalled (k=2, only 2 episodes exist), so Recall.Conflict must be
// true, and the grounded answer must still be returned (not suppressed).
func TestAskValenceConflictWithoutSuppressingGroundedAnswer(t *testing.T) {
	b := New(fakes.FakeEmbedding{}, nil, "ns", nil, 10)
	b.Record("reviews before merging catch bugs early", &Outcome{Reward: 1, Baseline: 0})
	b.Record("reviews before merging waste everyone time", &Outcome{Reward: 0, Baseline: 1})

	r := b.Ask("do reviews before merging actually help", 2)

	if !r.Grounded {
		t.Fatalf("expected Grounded=true even with conflicting valence, got false (reason=%q)", r.Reason)
	}
	if !r.Conflict {
		t.Fatalf("expected Conflict=true when co-recalled episodes have opposite valence")
	}
	if len(r.Episodes) != 2 {
		t.Fatalf("expected both conflicting episodes to be recalled, got %d", len(r.Episodes))
	}
}

// TestReappraiseAppendsPriorOutcomeAndSwapsIn: reappraising must preserve the
// old verdict (audit trail), not overwrite/lose it.
func TestReappraiseAppendsPriorOutcomeAndSwapsIn(t *testing.T) {
	b := New(fakes.FakeEmbedding{}, nil, "ns", nil, 10)
	oldOutcome := &Outcome{Reward: 1, Baseline: 0, Note: "seemed fine at the time"}
	ep := b.Record("shipped the migration without a backup", oldOutcome)

	newOutcome := Outcome{Reward: -5, Baseline: 0, Note: "actually corrupted prod data"}
	if !b.Reappraise(ep.ID, newOutcome) {
		t.Fatalf("expected Reappraise to succeed for a known id")
	}

	var got Episode
	found := false
	for _, e := range b.store.Episodes() {
		if e.ID == ep.ID {
			got = e
			found = true
		}
	}
	if !found {
		t.Fatalf("episode %q vanished from the store", ep.ID)
	}
	if got.Outcome == nil || *got.Outcome != newOutcome {
		t.Fatalf("expected Outcome swapped to the new outcome, got %+v", got.Outcome)
	}
	if len(got.PriorOutcomes) != 1 || got.PriorOutcomes[0] != *oldOutcome {
		t.Fatalf("expected old outcome preserved in PriorOutcomes, got %+v", got.PriorOutcomes)
	}
}

// TestReappraiseUnknownIDReturnsFalse: reappraising an id that isn't in the
// store is a no-op that reports failure, not a panic or silent insert.
func TestReappraiseUnknownIDReturnsFalse(t *testing.T) {
	b := New(fakes.FakeEmbedding{}, nil, "ns", nil, 10)
	if b.Reappraise("ep_does_not_exist", Outcome{Reward: 1}) {
		t.Fatalf("expected Reappraise on an unknown id to return false")
	}
}

// TestSetObjectiveDormancyRoundTrip: switching away from a conviction's goal
// frame dormants it; switching back revives it.
func TestSetObjectiveDormancyRoundTrip(t *testing.T) {
	b := NewWithStore(fakes.FakeEmbedding{}, nil, "ns", nil, 10, NewInMemoryStore(), nil)

	b.SetObjective("goal-a", "agent1")

	cv := Conviction{
		ID:            ConvictionID("ns", "always test before deploy", []string{"ep_x"}),
		Namespace:     "ns",
		Statement:     "always test before deploy",
		Valence:       "positive",
		SupportingIDs: []string{"ep_x"},
		SupportCount:  2,
		Consistency:   1.0,
		Confidence:    ConfidenceOf(2, 1.0),
		GoalFrame:     "goal-a",
	}
	b.store.PutConviction(cv)

	// Switch to goal-b: the goal-a-framed conviction should go dormant.
	b.SetObjective("goal-b", "agent1")

	found := false
	for _, c := range b.Convictions(true) {
		if c.ID == cv.ID {
			found = true
			if !c.Dormant {
				t.Fatalf("expected conviction to be Dormant after switching away from its goal frame")
			}
		}
	}
	if !found {
		t.Fatalf("conviction disappeared instead of going dormant (should survive under includeDormant=true)")
	}
	for _, c := range b.Convictions(false) {
		if c.ID == cv.ID {
			t.Fatalf("dormant conviction must not appear in Convictions(false)")
		}
	}

	// Switch back to goal-a: it should revive.
	b.SetObjective("goal-a", "agent1")

	revived := false
	for _, c := range b.Convictions(false) {
		if c.ID == cv.ID {
			revived = true
			if c.Dormant {
				t.Fatalf("expected conviction to be revived (Dormant=false) after returning to its goal frame")
			}
		}
	}
	if !revived {
		t.Fatalf("expected revived conviction to appear in Convictions(false)")
	}
}

// TestConsolidateFormsConvictionFromRepeatedConsistentEpisodes: repeated,
// consistent episodes should promote into at least one conviction, and the
// report's TotalConvictions must match what Convictions(false) actually returns.
func TestConsolidateFormsConvictionFromRepeatedConsistentEpisodes(t *testing.T) {
	b := NewWithStore(fakes.FakeEmbedding{}, nil, "ns", nil, 10, NewInMemoryStore(), nil)

	b.Record("standup meetings waste everyone time daily", &Outcome{Reward: 1, Baseline: 0})
	b.Record("standup meetings waste everyone time again", &Outcome{Reward: 1, Baseline: 0})
	b.Record("standup meetings waste everyone time still", &Outcome{Reward: 1, Baseline: 0})

	report := b.Consolidate(2, 0.6, 2, false, 0)

	if report.ConvictionsFormed < 1 {
		t.Fatalf("expected at least 1 conviction formed from repeated consistent episodes, got %+v", report)
	}
	if report.TotalConvictions != len(b.Convictions(false)) {
		t.Fatalf("TotalConvictions %d != len(Convictions(false)) %d", report.TotalConvictions, len(b.Convictions(false)))
	}
}

// TestConsolidateDoesNotDeleteExistingConvictionOnLaterConflict is the
// regression test the CEO asked for (CEO-BUG-REPORT.md, 2026-07-13): after a
// conviction has formed, a later consolidation that finds a CONFLICT (mixed
// valence in a cluster) must NOT zero out or delete the existing conviction —
// the brain must not "forget exactly when it learns more". Root cause of the
// live symptom was dormancy-visibility (an objective edit dormanted it), not
// deletion; this pins that consolidate itself never removes a conviction.
func TestConsolidateDoesNotDeleteExistingConvictionOnLaterConflict(t *testing.T) {
	b := NewWithStore(fakes.FakeEmbedding{}, nil, "ns", nil, 10, NewInMemoryStore(), nil)
	b.SetObjective("ship one paying customer per day", "ceo")

	// Three consistent negatives → one conviction.
	b.Record("overtrading in chop bled the account", &Outcome{Reward: -0.9, Baseline: 0})
	b.Record("overtrading again in chop bled the account", &Outcome{Reward: -0.9, Baseline: 0})
	b.Record("overtrading in chop keeps bleeding the account", &Outcome{Reward: -0.85, Baseline: 0})
	r1 := b.Consolidate(2, 0.66, 1, false, 0)
	if r1.ConvictionsFormed != 1 || len(b.Convictions(false)) != 1 {
		t.Fatalf("setup: expected exactly one conviction, got formed=%d active=%d", r1.ConvictionsFormed, len(b.Convictions(false)))
	}
	firstID := b.Convictions(false)[0].ID

	// Now add positive-reward episodes sharing the same tokens → the cluster
	// becomes mixed-valence and consolidation reports a conflict.
	b.Record("overtrading in chop actually made the account money", &Outcome{Reward: 0.9, Baseline: 0})
	b.Record("overtrading in chop grew the account nicely", &Outcome{Reward: 0.88, Baseline: 0})
	b.Record("overtrading in chop was great for the account", &Outcome{Reward: 0.9, Baseline: 0})
	r2 := b.Consolidate(2, 0.66, 1, false, 0)

	if r2.ConflictsSurfaced < 1 {
		t.Fatalf("expected the mixed-valence cluster to surface a conflict, got %+v", r2)
	}
	if r2.TotalConvictions == 0 || len(b.Convictions(false)) == 0 {
		t.Fatalf("BUG: a later conflict zeroed out convictions (total=%d active=%d) — the brain must not forget when it learns more", r2.TotalConvictions, len(b.Convictions(false)))
	}
	present := false
	for _, cv := range b.Convictions(true) { // include dormant: prove it was neither deleted nor silently dropped
		if cv.ID == firstID {
			present = true
		}
	}
	if !present {
		t.Fatalf("BUG: the original conviction %s disappeared from the store after a conflicting consolidate", firstID)
	}
}

// TestConsolidateForgetsStaleUnsupportedEpisodes exercises the forget path: a
// stale (past forgetAfterHours), unsupported (no cluster claimed it), non-
// positive-salience (nil outcome) episode must decay.
func TestConsolidateForgetsStaleUnsupportedEpisodes(t *testing.T) {
	clock := 0.0
	now := func() float64 { return clock }
	b := NewWithStore(fakes.FakeEmbedding{}, nil, "ns", nil, 10, NewInMemoryStore(), now)

	stale := b.Record("an old one-off note nobody ever revisited", nil)
	if !stale.Active() {
		t.Fatalf("sanity: freshly recorded episode should be active")
	}

	clock = 2 * 3600 // 2 hours later

	report := b.Consolidate(2, 0.6, 2, true, 1.0)
	if report.EpisodesDecayed < 1 {
		t.Fatalf("expected at least 1 episode decayed, got %+v", report)
	}

	found := false
	for _, e := range b.store.Episodes() {
		if e.ID == stale.ID {
			found = true
			if e.Active() {
				t.Fatalf("expected stale, unsupported, non-positive-salience episode to become inactive")
			}
		}
	}
	if !found {
		t.Fatalf("episode vanished from the store")
	}
}

// TestFileStoreRoundTrip: episodes, objective, and convictions written via one
// FileStore value survive a fresh NewFileStore load on the same directory,
// including version counters continuing from where they left off.
func TestFileStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()

	fs1, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore: %v", err)
	}
	b := NewWithStore(fakes.FakeEmbedding{}, nil, "ns", nil, 10, fs1, nil)

	ep := b.Record("persisted across reloads", &Outcome{Reward: 1, Baseline: 0})
	b.SetObjective("goal-a", "agent1")

	cv := Conviction{
		ID:            ConvictionID("ns", "persisted belief", []string{ep.ID}),
		Namespace:     "ns",
		Statement:     "persisted belief",
		Valence:       "positive",
		SupportingIDs: []string{ep.ID},
		SupportCount:  2,
		Consistency:   1.0,
		Confidence:    ConfidenceOf(2, 1.0),
		GoalFrame:     "goal-a",
	}
	fs1.PutConviction(cv)

	fs2, err := NewFileStore(dir)
	if err != nil {
		t.Fatalf("NewFileStore (reload): %v", err)
	}

	eps := fs2.Episodes()
	if len(eps) != 1 || eps[0].ID != ep.ID || eps[0].Text != ep.Text {
		t.Fatalf("expected episode to survive reload, got %+v", eps)
	}

	obj, ok := fs2.CurrentObjective()
	if !ok || obj.Text != "goal-a" {
		t.Fatalf("expected objective to survive reload, got %+v ok=%v", obj, ok)
	}

	cvs := fs2.Convictions()
	if len(cvs) != 1 || cvs[0].ID != cv.ID {
		t.Fatalf("expected conviction to survive reload, got %+v", cvs)
	}

	if got := fs2.NextVersion(); got != ep.Version+1 {
		t.Fatalf("expected NextVersion after reload to continue from persisted max version, got %d want %d", got, ep.Version+1)
	}
	if got := fs2.NextObjectiveVersion(); got != 2 {
		t.Fatalf("expected NextObjectiveVersion after reload to continue from persisted version, got %d want 2", got)
	}
}

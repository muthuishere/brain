package engine

import (
	"strings"
	"testing"
)

// hardConstraint is a Hard-kind constraint whose cost is a fixed value read via
// Check, so it deterministically violates (cost > threshold) whenever wantViolate
// is true.
func costConstraint(name string, kind ConstraintKind, threshold, weight, cost float64) Constraint {
	return Constraint{
		Name:      name,
		Text:      name + " text",
		Kind:      kind,
		Threshold: threshold,
		Weight:    weight,
		Check:     func(DecisionContext) float64 { return cost },
	}
}

func TestShield_HardConstraintVetoesRegardlessOfReward(t *testing.T) {
	hard := costConstraint("no-ruin", Hard, 0.5, 1.0, 0.9) // cost 0.9 > threshold 0.5 -> violated
	shield := Shield{constraints: []Constraint{hard}, highReward: 10}
	ctx := DecisionContext{Text: "do the risky thing"}

	for _, reward := range []float64{-100, 0, 1, 1000} {
		t.Run("", func(t *testing.T) {
			v := shield.Evaluate(ctx, reward, "safe-fallback")
			if v.Allowed {
				t.Fatalf("reward=%v: expected Allowed=false, got true", reward)
			}
			found := false
			for _, n := range v.VetoedBy {
				if n == "no-ruin" {
					found = true
				}
			}
			if !found {
				t.Fatalf("reward=%v: expected VetoedBy to contain %q, got %v", reward, "no-ruin", v.VetoedBy)
			}
			if v.Fallback != "safe-fallback" {
				t.Fatalf("reward=%v: expected Fallback=%q, got %q", reward, "safe-fallback", v.Fallback)
			}
		})
	}
}

func TestShield_SoftConstraintPenalizesNotBlocks(t *testing.T) {
	threshold := 0.3
	weight := 2.0
	cost := 0.8
	soft := costConstraint("mild-strain", Soft, threshold, weight, cost)
	shield := Shield{constraints: []Constraint{soft}, highReward: 100}
	ctx := DecisionContext{Text: "borderline decision"}
	objectiveReward := 10.0

	v := shield.Evaluate(ctx, objectiveReward, "fallback")

	if !v.Allowed {
		t.Fatalf("expected Allowed=true for a soft-only violation, got false")
	}
	found := false
	for _, n := range v.PenalizedBy {
		if n == "mild-strain" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected PenalizedBy to contain %q, got %v", "mild-strain", v.PenalizedBy)
	}
	wantPenalty := weight * (cost - threshold)
	wantAdjusted := objectiveReward - wantPenalty
	if v.AdjustedReward != wantAdjusted {
		t.Fatalf("expected AdjustedReward=%v (ObjectiveReward - Weight*(cost-threshold)=%v), got %v",
			wantAdjusted, wantPenalty, v.AdjustedReward)
	}
	if !(v.AdjustedReward < v.ObjectiveReward) {
		t.Fatalf("expected AdjustedReward < ObjectiveReward, got adjusted=%v objective=%v", v.AdjustedReward, v.ObjectiveReward)
	}
}

func TestShield_AlarmIsConjunctionOfHighRewardAndViolation(t *testing.T) {
	const highReward = 50.0
	violated := costConstraint("violator", Soft, 0.1, 1.0, 0.9) // always violates
	clean := costConstraint("clean", Soft, 10.0, 1.0, 0.0)      // never violates (cost 0 <= threshold 10)

	cases := []struct {
		name        string
		reward      float64
		constraints []Constraint
		wantAlarm   bool
	}{
		{"high reward + violation", 60, []Constraint{violated}, true},
		{"high reward + no violation", 60, []Constraint{clean}, false},
		{"low reward + violation", 10, []Constraint{violated}, false},
		{"low reward + no violation", 10, []Constraint{clean}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shield := Shield{constraints: tc.constraints, highReward: highReward}
			v := shield.Evaluate(DecisionContext{Text: "x"}, tc.reward, "fallback")
			if v.Alarm != tc.wantAlarm {
				t.Fatalf("reward=%v: expected Alarm=%v, got %v (reasons=%v)", tc.reward, tc.wantAlarm, v.Alarm, v.Reasons)
			}
			if tc.wantAlarm {
				if len(v.Reasons) == 0 || !strings.Contains(v.Reasons[0], "profitable-but-ruinous") {
					t.Fatalf("expected the alarm reason to be prepended, got %v", v.Reasons)
				}
			}
		})
	}
}

// Boundary case: reward exactly at highReward counts as "high" (>=).
func TestShield_AlarmBoundaryAtHighReward(t *testing.T) {
	violated := costConstraint("violator", Soft, 0.1, 1.0, 0.9)
	shield := Shield{constraints: []Constraint{violated}, highReward: 50}
	v := shield.Evaluate(DecisionContext{Text: "x"}, 50, "fallback")
	if !v.Alarm {
		t.Fatalf("expected Alarm=true when reward == highReward exactly, got false")
	}
}

func TestVerdict_GuaranteedTrueWhenAllEvaluationsDeterministic(t *testing.T) {
	checkBased := costConstraint("checked", Soft, 1.0, 1.0, 0.0) // Check-sourced, deterministic
	signalBased := Constraint{
		Name:      "signalled",
		Text:      "signalled text",
		Kind:      Soft,
		Threshold: 1.0,
		Weight:    1.0,
		Signal:    "risk", // Signal-sourced; cost() marks this deterministic=true too
	}
	shield := Shield{constraints: []Constraint{checkBased, signalBased}, highReward: 100}
	ctx := DecisionContext{Text: "x", Signals: map[string]float64{"risk": 0.2}}

	v := shield.Evaluate(ctx, 1, "fallback")

	if len(v.Evaluations) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(v.Evaluations))
	}
	for _, e := range v.Evaluations {
		if !e.Deterministic {
			t.Fatalf("expected evaluation %q to be Deterministic, got false", e.Name)
		}
	}
	if !v.Guaranteed() {
		t.Fatalf("expected Guaranteed()=true when every constraint is Check- or Signal-sourced")
	}
}

func TestVerdict_GuaranteedFalseWhenAnyEvaluationUndetermined(t *testing.T) {
	checkBased := costConstraint("checked", Soft, 1.0, 1.0, 0.0)
	undetermined := Constraint{
		Name:      "undetermined",
		Text:      "neither Check nor Signal set",
		Kind:      Soft,
		Threshold: 1.0,
		Weight:    1.0,
		// Neither Check nor Signal set -> cost() returns (0, false).
	}
	shield := Shield{constraints: []Constraint{checkBased, undetermined}, highReward: 100}
	ctx := DecisionContext{Text: "x"}

	v := shield.Evaluate(ctx, 1, "fallback")

	if len(v.Evaluations) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(v.Evaluations))
	}
	if v.Guaranteed() {
		t.Fatalf("expected Guaranteed()=false when at least one evaluated constraint has neither Check nor Signal")
	}
}

func TestVerdict_GuaranteedTrueWhenNoConstraints(t *testing.T) {
	shield := Shield{constraints: nil, highReward: 100}
	v := shield.Evaluate(DecisionContext{Text: "x"}, 1, "fallback")
	if len(v.Evaluations) != 0 {
		t.Fatalf("expected no evaluations, got %d", len(v.Evaluations))
	}
	if !v.Guaranteed() {
		t.Fatalf("expected Guaranteed()=true (vacuously) when there are no evaluated constraints")
	}
}

func TestShield_MixedHardAndSoft_HardVetoesButPenaltyStillAccrues(t *testing.T) {
	hardThreshold, hardCost := 0.5, 0.9
	hard := costConstraint("hard-veto", Hard, hardThreshold, 1.0, hardCost)

	softThreshold, softWeight, softCost := 0.2, 3.0, 0.6
	soft := costConstraint("soft-penalty", Soft, softThreshold, softWeight, softCost)

	shield := Shield{constraints: []Constraint{hard, soft}, highReward: 1000}
	objectiveReward := 20.0

	v := shield.Evaluate(DecisionContext{Text: "mixed"}, objectiveReward, "the-fallback")

	if v.Allowed {
		t.Fatalf("expected Allowed=false: hard constraint must win over soft")
	}
	if v.Fallback != "the-fallback" {
		t.Fatalf("expected Fallback=%q, got %q", "the-fallback", v.Fallback)
	}

	wantPenalty := softWeight * (softCost - softThreshold)
	wantAdjusted := objectiveReward - wantPenalty
	if v.AdjustedReward != wantAdjusted {
		t.Fatalf("expected AdjustedReward=%v (penalty math must still run), got %v", wantAdjusted, v.AdjustedReward)
	}

	hardFound := false
	for _, n := range v.VetoedBy {
		if n == "hard-veto" {
			hardFound = true
		}
	}
	if !hardFound {
		t.Fatalf("expected VetoedBy to contain %q, got %v", "hard-veto", v.VetoedBy)
	}

	softFound := false
	for _, n := range v.PenalizedBy {
		if n == "soft-penalty" {
			softFound = true
		}
	}
	if !softFound {
		t.Fatalf("expected PenalizedBy to contain %q, got %v", "soft-penalty", v.PenalizedBy)
	}
}

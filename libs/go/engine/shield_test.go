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
	checkBased := costConstraint("checked", Soft, 1.0, 1.0, 0.0) // Check-sourced -> Computed
	signalBased := Constraint{
		Name:      "signalled",
		Text:      "signalled text",
		Kind:      Soft,
		Threshold: 1.0,
		Weight:    1.0,
		Signal:    "risk", // Signal supplied in ctx -> Provided
	}
	shield := Shield{constraints: []Constraint{checkBased, signalBased}, highReward: 100}
	ctx := DecisionContext{Text: "x", Signals: map[string]float64{"risk": 0.2}}

	v := shield.Evaluate(ctx, 1, "fallback")

	if len(v.Evaluations) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(v.Evaluations))
	}
	for _, e := range v.Evaluations {
		if !e.Deterministic() {
			t.Fatalf("expected evaluation %q to be Deterministic, got false (provenance=%v)", e.Name, e.Provenance)
		}
	}
	if !v.Guaranteed() {
		t.Fatalf("expected Guaranteed()=true when every constraint is Computed- or Provided-sourced")
	}
}

func TestVerdict_GuaranteedFalseWhenAnyEvaluationUndetermined(t *testing.T) {
	checkBased := costConstraint("checked", Soft, 1.0, 1.0, 0.0)
	unbound := Constraint{
		Name:      "unbound",
		Text:      "neither Check nor Signal set",
		Kind:      Soft,
		Threshold: 1.0,
		Weight:    1.0,
		// Neither Check nor Signal set -> cost() returns (0, Unbound).
	}
	shield := Shield{constraints: []Constraint{checkBased, unbound}, highReward: 100}
	ctx := DecisionContext{Text: "x"}

	v := shield.Evaluate(ctx, 1, "fallback")

	if len(v.Evaluations) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(v.Evaluations))
	}
	if v.Guaranteed() {
		t.Fatalf("expected Guaranteed()=false when at least one evaluated constraint has neither Check nor Signal")
	}
	// Unbound always behaves as assume_safe: never blocks, regardless of kind.
	if !v.Allowed {
		t.Fatalf("expected Allowed=true: an Unbound constraint must not block (it cannot fail closed on nothing)")
	}
	if v.Undetermined {
		t.Fatalf("expected Verdict.Undetermined=false for a Soft Unbound constraint (only Hard absent-signal fails closed)")
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

// --- SPEC-shield-signal-provenance-v1 §6: the regression suite for the
// fail-closed fix. never-ruin mirrors the SPEC's own worked example: a Hard
// constraint that reads its cost from a "ruin_risk" signal the caller supplies.

func neverRuin(whenAbsent string) Constraint {
	return Constraint{
		Name: "never-ruin", Text: "never risk ruin", Kind: Hard,
		Threshold: 0.5, Weight: 1.0, Signal: "ruin_risk", WhenAbsent: whenAbsent,
	}
}

// 1. Present-and-safe: the signal is supplied and 0 -> allowed, guaranteed,
// determined -- distinguishing a genuinely safe measurement from a missing one.
func TestShield_SignalPresentAndSafe(t *testing.T) {
	shield := Shield{constraints: []Constraint{neverRuin("")}, highReward: 10}
	v := shield.Evaluate(DecisionContext{Text: "x", Signals: map[string]float64{"ruin_risk": 0}}, 1, "fallback")

	if !v.Allowed {
		t.Fatalf("expected Allowed=true, got false (reasons=%v)", v.Reasons)
	}
	if !v.Guaranteed() {
		t.Fatalf("expected Guaranteed()=true: the signal was actually supplied")
	}
	if v.Undetermined {
		t.Fatalf("expected Undetermined=false, got true")
	}
}

// 2. Present-and-violating: the signal is supplied and over threshold -> veto.
func TestShield_SignalPresentAndViolating(t *testing.T) {
	shield := Shield{constraints: []Constraint{neverRuin("")}, highReward: 10}
	v := shield.Evaluate(DecisionContext{Text: "x", Signals: map[string]float64{"ruin_risk": 1}}, 1, "fallback")

	if v.Allowed {
		t.Fatalf("expected Allowed=false, got true")
	}
	if len(v.VetoedBy) != 1 || v.VetoedBy[0] != "never-ruin" {
		t.Fatalf("expected VetoedBy=[never-ruin], got %v", v.VetoedBy)
	}
}

// 3. THE BUG'S REGRESSION TEST: a hard constraint's signal is simply omitted.
// Pre-fix this silently returned allowed:true, guaranteed:true. It must now
// fail closed: undetermined, not allowed, and honestly not guaranteed.
func TestShield_SignalAbsent_DefaultVetoFailsClosed(t *testing.T) {
	shield := Shield{constraints: []Constraint{neverRuin("")}, highReward: 10}
	v := shield.Evaluate(DecisionContext{Text: "bet the account"}, 0.95, "safe-fallback")

	if v.Allowed {
		t.Fatalf("REGRESSION: an omitted required signal must not silently pass, got Allowed=true")
	}
	if !v.Undetermined {
		t.Fatalf("expected Undetermined=true")
	}
	if len(v.UndeterminedBy) != 1 || v.UndeterminedBy[0] != "never-ruin" {
		t.Fatalf("expected UndeterminedBy=[never-ruin], got %v", v.UndeterminedBy)
	}
	if len(v.VetoedBy) != 0 {
		t.Fatalf("expected VetoedBy to stay empty (nothing was actually measured as violating), got %v", v.VetoedBy)
	}
	if v.Guaranteed() {
		t.Fatalf("REGRESSION: an undetermined hard constraint must not report Guaranteed()=true")
	}
	if v.Fallback != "safe-fallback" {
		t.Fatalf("expected Fallback to be populated when the decision is blocked, got %q", v.Fallback)
	}
}

// 4. Absent + high reward: undetermined is itself alarm-worthy — a high-stakes
// decision with its safety input missing must not pass quietly.
func TestShield_SignalAbsent_HighRewardAlarms(t *testing.T) {
	shield := Shield{constraints: []Constraint{neverRuin("")}, highReward: 0.5}
	v := shield.Evaluate(DecisionContext{Text: "x"}, 0.95, "fallback")
	if !v.Alarm {
		t.Fatalf("expected Alarm=true when reward is high and a hard constraint is undetermined")
	}
}

// 5. Absent + explicit assume_safe opt-in on a Hard constraint: the escape
// hatch is honored, but Guaranteed() still tells the truth about it.
func TestShield_SignalAbsent_AssumeSafeOptIn(t *testing.T) {
	shield := Shield{constraints: []Constraint{neverRuin(WhenAbsentAssumeSafe)}, highReward: 10}
	v := shield.Evaluate(DecisionContext{Text: "x"}, 1, "fallback")

	if !v.Allowed {
		t.Fatalf("expected Allowed=true (assume_safe opt-in), got false")
	}
	if v.Undetermined {
		t.Fatalf("expected Undetermined=false under assume_safe, got true")
	}
	if v.Guaranteed() {
		t.Fatalf("expected Guaranteed()=false: the cost was assumed, not measured")
	}
}

// 6. Soft absent (default policy is assume_safe for Soft): no penalty accrues,
// but the verdict is honestly non-guaranteed.
func TestShield_SoftSignalAbsent_DefaultAssumeSafeNoPenalty(t *testing.T) {
	soft := Constraint{
		Name: "prefer-diversified", Text: "avoid concentration", Kind: Soft,
		Threshold: 0.7, Weight: 1.0, Signal: "concentration",
	}
	shield := Shield{constraints: []Constraint{soft}, highReward: 100}
	v := shield.Evaluate(DecisionContext{Text: "x"}, 1, "fallback")

	if len(v.PenalizedBy) != 0 {
		t.Fatalf("expected no penalty for an absent soft signal (assume_safe default), got %v", v.PenalizedBy)
	}
	if v.AdjustedReward != v.ObjectiveReward {
		t.Fatalf("expected AdjustedReward == ObjectiveReward (no penalty applied), got %v vs %v", v.AdjustedReward, v.ObjectiveReward)
	}
	if v.Guaranteed() {
		t.Fatalf("expected Guaranteed()=false: the soft constraint's cost was never measured")
	}
}

// 9. Determinism: identical inputs must produce identical verdicts, and the
// name lists must be sorted (a guard against any future map-iteration drift).
func TestShield_DeterministicAndSorted(t *testing.T) {
	constraints := []Constraint{
		{Name: "z-first", Text: "z", Kind: Hard, Threshold: 0.5, Weight: 1, Signal: "z"},
		{Name: "a-second", Text: "a", Kind: Hard, Threshold: 0.5, Weight: 1, Signal: "a"},
	}
	shield := Shield{constraints: constraints, highReward: 10}
	ctx := DecisionContext{Text: "x"} // both signals absent -> both undetermined

	first := shield.Evaluate(ctx, 1, "fallback")
	for i := 0; i < 5; i++ {
		v := shield.Evaluate(ctx, 1, "fallback")
		if v.Allowed != first.Allowed || v.Undetermined != first.Undetermined ||
			len(v.UndeterminedBy) != len(first.UndeterminedBy) {
			t.Fatalf("run %d: verdict differs from first run", i)
		}
		for j := range v.UndeterminedBy {
			if v.UndeterminedBy[j] != first.UndeterminedBy[j] {
				t.Fatalf("run %d: UndeterminedBy order differs: %v vs %v", i, v.UndeterminedBy, first.UndeterminedBy)
			}
		}
	}
	want := []string{"a-second", "z-first"}
	if len(first.UndeterminedBy) != 2 || first.UndeterminedBy[0] != want[0] || first.UndeterminedBy[1] != want[1] {
		t.Fatalf("expected UndeterminedBy sorted %v, got %v", want, first.UndeterminedBy)
	}
}

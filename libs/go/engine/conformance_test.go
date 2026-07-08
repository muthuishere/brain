package engine

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// conformance/cases/shield.json is the language-neutral golden-vector fixture
// (docs/SPEC-shield-conformance-v1.md): declarative constraints + a decision's
// signals -> expected Verdict fields. This test proves the Go engine — the
// only implementation that exists today — matches every vector exactly, and
// gives any future port's own runner an identical file to read.

type conformanceConstraint struct {
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	Signal     string  `json:"signal"`
	Threshold  float64 `json:"threshold"`
	Weight     float64 `json:"weight"`
	WhenAbsent string  `json:"when_absent"`
}

type conformanceExpect struct {
	Allowed        bool     `json:"allowed"`
	Alarm          bool     `json:"alarm"`
	Undetermined   bool     `json:"undetermined"`
	UndeterminedBy []string `json:"undetermined_by"`
	VetoedBy       []string `json:"vetoed_by"`
	PenalizedBy    []string `json:"penalized_by"`
	Guaranteed     bool     `json:"guaranteed"`
	AdjustedReward *float64 `json:"adjusted_reward"`
}

type conformanceCase struct {
	Name            string                  `json:"name"`
	Constraints     []conformanceConstraint `json:"constraints"`
	Signals         map[string]float64      `json:"signals"`
	ObjectiveReward float64                 `json:"objective_reward"`
	HighReward      float64                 `json:"high_reward"`
	Fallback        string                  `json:"fallback"`
	Expect          conformanceExpect       `json:"expect"`
}

func loadShieldConformanceCases(t *testing.T) []conformanceCase {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("could not determine test file location")
	}
	// libs/go/engine/conformance_test.go -> repo root is four levels up.
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	path := filepath.Join(repoRoot, "conformance", "cases", "shield.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading conformance fixture %s: %v", path, err)
	}
	var cases []conformanceCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatalf("parsing conformance fixture: %v", err)
	}
	if len(cases) == 0 {
		t.Fatal("conformance fixture has no cases")
	}
	return cases
}

func TestShieldConformance(t *testing.T) {
	for _, tc := range loadShieldConformanceCases(t) {
		t.Run(tc.Name, func(t *testing.T) {
			constraints := make([]Constraint, len(tc.Constraints))
			for i, c := range tc.Constraints {
				kind := Soft
				if c.Kind == "hard" {
					kind = Hard
				}
				constraints[i] = Constraint{
					Name: c.Name, Kind: kind, Signal: c.Signal,
					Threshold: c.Threshold, Weight: c.Weight, WhenAbsent: c.WhenAbsent,
				}
			}
			shield := Shield{constraints: constraints, highReward: tc.HighReward}
			ctx := DecisionContext{Text: tc.Name, Signals: tc.Signals}
			v := shield.Evaluate(ctx, tc.ObjectiveReward, tc.Fallback)

			if v.Allowed != tc.Expect.Allowed {
				t.Errorf("Allowed = %v, want %v", v.Allowed, tc.Expect.Allowed)
			}
			if v.Alarm != tc.Expect.Alarm {
				t.Errorf("Alarm = %v, want %v", v.Alarm, tc.Expect.Alarm)
			}
			if v.Undetermined != tc.Expect.Undetermined {
				t.Errorf("Undetermined = %v, want %v", v.Undetermined, tc.Expect.Undetermined)
			}
			if v.Guaranteed() != tc.Expect.Guaranteed {
				t.Errorf("Guaranteed() = %v, want %v", v.Guaranteed(), tc.Expect.Guaranteed)
			}
			assertStringSlice(t, "UndeterminedBy", v.UndeterminedBy, tc.Expect.UndeterminedBy)
			assertStringSlice(t, "VetoedBy", v.VetoedBy, tc.Expect.VetoedBy)
			assertStringSlice(t, "PenalizedBy", v.PenalizedBy, tc.Expect.PenalizedBy)

			wantAdjusted := tc.ObjectiveReward
			if tc.Expect.AdjustedReward != nil {
				wantAdjusted = *tc.Expect.AdjustedReward
			}
			if v.AdjustedReward != wantAdjusted {
				t.Errorf("AdjustedReward = %v, want %v", v.AdjustedReward, wantAdjusted)
			}
		})
	}
}

func assertStringSlice(t *testing.T, field string, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s = %v, want %v", field, got, want)
		return
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("%s = %v, want %v", field, got, want)
			return
		}
	}
}

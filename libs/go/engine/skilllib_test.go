package engine

import (
	"os"
	"path/filepath"
	"testing"
)

func newLib(t *testing.T) *SkillLib {
	t.Helper()
	l, err := LoadSkillLib(filepath.Join(t.TempDir(), "skills-lib"))
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestRegisterNewVersionLineage(t *testing.T) {
	l := newLib(t)
	if _, _, err := l.RegisterSkill("fetch-quotes", "primitive", "", "", "", "  ", "spec", 1); err == nil {
		t.Fatal("empty rationale must be rejected")
	}
	s, v, err := l.RegisterSkill("fetch-quotes", "primitive", "fetch quotes", "market-data", "", "initial", "spec v1", 1)
	if err != nil || v != 1 || s.CurrentVersion != 1 || !s.Active() {
		t.Fatalf("first register: v=%d err=%v skill=%+v", v, err, s)
	}
	// New version is a CANDIDATE — current stays put until validated.
	s, v, err = l.RegisterSkill("fetch-quotes", "primitive", "", "", "", "handle rate limits", "spec v2", 2)
	if err != nil || v != 2 || s.CurrentVersion != 1 || len(s.Versions) != 2 {
		t.Fatalf("second register: v=%d cur=%d err=%v", v, s.CurrentVersion, err)
	}
	if s.Versions[1].Rationale != "handle rate limits" {
		t.Fatalf("rationale lost: %+v", s.Versions[1])
	}
	for _, sv := range s.Versions {
		if _, err := os.Stat(filepath.Join(l.dir, sv.File)); err != nil {
			t.Fatalf("spec file missing: %s", sv.File)
		}
	}
	// Child skill keeps parent lineage.
	child, _, err := l.RegisterSkill("fetch-quotes-batch", "composite", "", "market-data", "fetch-quotes", "batch variant", "spec", 3)
	if err != nil || child.Parent != "fetch-quotes" || child.Kind != "composite" {
		t.Fatalf("lineage: %+v err=%v", child, err)
	}
	// Save/Load round-trips.
	if err := l.Save(); err != nil {
		t.Fatal(err)
	}
	l2, err := LoadSkillLib(l.dir)
	if err != nil || len(l2.Skills) != 2 {
		t.Fatalf("reload: %v skills=%d", err, len(l2.Skills))
	}
}

func TestValidateGate(t *testing.T) {
	cases := []struct {
		name             string
		prior, candidate TestResults
		accept           bool
	}{
		{"clear improvement", TestResults{Passed: 6, Failed: 4}, TestResults{Passed: 8, Failed: 2}, true},
		{"regression", TestResults{Passed: 8, Failed: 2}, TestResults{Passed: 6, Failed: 4}, false},
		{"exact threshold accepted", TestResults{Passed: 60, Failed: 40}, TestResults{Passed: 65, Failed: 35}, true},
		{"just under threshold", TestResults{Passed: 60, Failed: 40}, TestResults{Passed: 64, Failed: 36}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := newLib(t)
			l.RegisterSkill("s", "primitive", "", "", "", "init", "v1", 1)
			l.RegisterSkill("s", "primitive", "", "", "", "improve", "v2", 2)
			res, err := l.ValidateVersion("s", 2, tc.prior, tc.candidate, 0.05, 3)
			if err != nil {
				t.Fatal(err)
			}
			if res.Accepted != tc.accept {
				t.Fatalf("accepted=%v want %v (%+v)", res.Accepted, tc.accept, res)
			}
			s := l.Find("s")
			if tc.accept && s.CurrentVersion != 2 {
				t.Fatalf("accepted candidate not promoted: cur=%d", s.CurrentVersion)
			}
			if !tc.accept {
				if s.CurrentVersion != 1 {
					t.Fatalf("rejected candidate promoted: cur=%d", s.CurrentVersion)
				}
				sv := s.Versions[1]
				if sv.Validation == nil || sv.Validation.Accepted {
					t.Fatalf("rejection not recorded: %+v", sv)
				}
				if filepath.Dir(sv.File) != filepath.Join("archived", "s") {
					t.Fatalf("rejected file not archived: %s", sv.File)
				}
				if _, err := os.Stat(filepath.Join(l.dir, sv.File)); err != nil {
					t.Fatalf("archived file missing: %v", err)
				}
			}
		})
	}
}

func TestLogUsageRecomputesMetricsAndWindow(t *testing.T) {
	l := newLib(t)
	l.RegisterSkill("s", "primitive", "", "", "", "init", "v1", 1)
	for i, outcome := range []string{"ok", "ok", "fail", "ok"} {
		ev := UsageEvent{SkillID: "s", Task: "t", Outcome: outcome, Cost: 1, Ts: float64(10 * (i + 1))}
		if err := l.LogUsage(ev); err != nil {
			t.Fatal(err)
		}
	}
	m := l.Find("s").Metrics
	if m.InvocationCount != 4 || m.SuccessRate != 0.75 || m.TotalCost != 4 {
		t.Fatalf("metrics: %+v", m)
	}
	// Windowed: only the last two events (ts 30 fail, ts 40 ok).
	w, err := l.MetricsWindow("s", 25)
	if err != nil || w.InvocationCount != 2 || w.SuccessRate != 0.5 {
		t.Fatalf("window: %+v err=%v", w, err)
	}
	if err := l.LogUsage(UsageEvent{SkillID: "s", Outcome: "meh"}); err == nil {
		t.Fatal("bad outcome must error")
	}
	if err := l.LogUsage(UsageEvent{SkillID: "nope", Outcome: "ok"}); err == nil {
		t.Fatal("unknown skill must error")
	}
}

func TestSearchDeprecateRollback(t *testing.T) {
	l := newLib(t)
	l.RegisterSkill("quotes", "primitive", "fetch market quotes", "market-data", "", "init", "v1", 1)
	l.RegisterSkill("emails", "primitive", "draft emails", "comms", "", "init", "v1", 1)
	l.LogUsage(UsageEvent{SkillID: "quotes", Outcome: "ok", Ts: 1})
	if got := l.Search("market", 0); len(got) != 1 || got[0].ID != "quotes" {
		t.Fatalf("search domain: %+v", got)
	}
	if got := l.Search("", 0.5); len(got) != 1 || got[0].ID != "quotes" { // emails has 0 success
		t.Fatalf("search min-success: %+v", got)
	}
	if err := l.Deprecate("emails"); err != nil {
		t.Fatal(err)
	}
	if got := l.Search("", 0); len(got) != 1 {
		t.Fatalf("deprecated skill still searchable: %+v", got)
	}
	// Rollback: v2 accepted then rolled back to v1; rejected versions refuse.
	l.RegisterSkill("quotes", "primitive", "", "", "", "v2", "spec2", 2)
	if _, err := l.ValidateVersion("quotes", 2, TestResults{Passed: 1, Failed: 9}, TestResults{Passed: 9, Failed: 1}, 0.05, 3); err != nil {
		t.Fatal(err)
	}
	if err := l.Rollback("quotes", 1); err != nil || l.Find("quotes").CurrentVersion != 1 {
		t.Fatalf("rollback: %v cur=%d", err, l.Find("quotes").CurrentVersion)
	}
	l.RegisterSkill("quotes", "primitive", "", "", "", "v3 bad", "spec3", 4)
	l.ValidateVersion("quotes", 3, TestResults{Passed: 9, Failed: 1}, TestResults{Passed: 1, Failed: 9}, 0.05, 5)
	if err := l.Rollback("quotes", 3); err == nil {
		t.Fatal("rollback to a gate-rejected version must fail")
	}
	// Deprecate + rollback revives.
	l.Deprecate("quotes")
	if err := l.Rollback("quotes", 2); err != nil || l.Find("quotes").Status != "active" {
		t.Fatalf("rollback should revive: %v", err)
	}
}

func TestRegisterRejectsPathLikeIDs(t *testing.T) {
	l := newLib(t)
	for _, id := range []string{"", ".", "..", "../evil", "a/b", `a\b`, "../../evolve-policy.json"} {
		if _, _, err := l.RegisterSkill(id, "primitive", "d", "dom", "", "why", "spec", 1); err == nil {
			t.Fatalf("id %q should be rejected", id)
		}
	}
}

func TestValidateRefusesCurrentVersion(t *testing.T) {
	l := newLib(t)
	if _, _, err := l.RegisterSkill("solo", "primitive", "d", "dom", "", "why", "spec", 1); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ValidateVersion("solo", 1, TestResults{Passed: 1}, TestResults{}, 0.05, 2); err == nil {
		t.Fatal("validating the current (sole) version should be refused")
	}
	s := l.Find("solo")
	if s.CurrentVersion != 1 || s.Versions[0].File == "" || s.Versions[0].Validation != nil {
		t.Fatalf("live version must be untouched: %+v", s.Versions[0])
	}
}

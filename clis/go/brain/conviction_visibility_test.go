package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// formConviction drives the real CEO learning path over the CLI surface: set an
// objective, record two consistent negative episodes that share content tokens,
// then consolidate. With cmdConsolidate's defaults (minSupport 2, minConsistency
// 0.66, minShared 1) this promotes exactly one conviction framed on the current
// objective — the substrate the visibility tests need.
func formConviction(t *testing.T, repo, objective string) {
	t.Helper()
	cmdInit(repo)
	cmdObjective(repo, []string{objective})
	cmdRecord(repo, []string{"overtrading in chop bled the account", "--reward", "-0.6"})
	cmdRecord(repo, []string{"overtrading again in chop bled the account badly", "--reward", "-0.5"})
	out := captureStdout(t, func() { cmdConsolidate(repo, nil) })
	if !strings.Contains(out, "convictions formed: 1") {
		t.Fatalf("setup: expected one conviction to form, got: %s", out)
	}
}

// TestConvictionVisibilityAfterObjectiveEdit is the CLI-level regression test for
// the fundamentals bug found against the live CEO brain: editing the objective
// silently dormants the brain's convictions, and status/convictions then gave NO
// indication one existed. After the fix a dormant conviction must remain visible
// (status count + `convictions --all`), while the default listing stays active-only.
func TestConvictionVisibilityAfterObjectiveEdit(t *testing.T) {
	repo := t.TempDir()
	formConviction(t, repo, "ship one paying customer per day")

	// Active before the edit: status shows a plain count, no dormant note.
	before := captureStdout(t, func() { cmdStatus(repo) })
	if !strings.Contains(before, "convictions : 1") || strings.Contains(before, "dormant") {
		t.Fatalf("pre-edit status should show 1 active conviction, no dormant note; got:\n%s", before)
	}

	// Edit the objective — the conviction's goal frame retires, so it dormants.
	cmdObjective(repo, []string{"FUNDAMENTALS-FIRST MODE: fix the organs before anything else"})

	after := captureStdout(t, func() { cmdStatus(repo) })
	if !strings.Contains(after, "0 active") || !strings.Contains(after, "1 dormant") {
		t.Fatalf("post-edit status must reveal the dormant conviction (0 active, 1 dormant); got:\n%s", after)
	}

	// Default `convictions` stays active-only (unchanged behavior).
	def := captureStdout(t, func() { cmdConvictions(repo, nil) })
	if !strings.Contains(def, "no convictions yet") {
		t.Fatalf("default convictions listing should be active-only after the edit; got:\n%s", def)
	}

	// `convictions --all` surfaces the dormant conviction, labeled.
	all := captureStdout(t, func() { cmdConvictions(repo, []string{"--all"}) })
	if !strings.Contains(all, "[dormant]") || !strings.Contains(all, "overtrading") {
		t.Fatalf("`convictions --all` must show the dormant conviction labeled [dormant]; got:\n%s", all)
	}

	// Returning to the original objective revives it.
	cmdObjective(repo, []string{"ship one paying customer per day"})
	revived := captureStdout(t, func() { cmdStatus(repo) })
	if !strings.Contains(revived, "convictions : 1") || strings.Contains(revived, "dormant") {
		t.Fatalf("returning to the frame should revive the conviction to active; got:\n%s", revived)
	}
}

// TestShieldVetoIndependentOfConvictionDormancy proves the safety mechanism is
// not entangled with conviction visibility: a hard-constraint violation is still
// vetoed even when every conviction is dormant.
func TestShieldVetoIndependentOfConvictionDormancy(t *testing.T) {
	repo := t.TempDir()
	formConviction(t, repo, "ship one paying customer per day")
	cmdObjective(repo, []string{"a different objective entirely"}) // dormant everything

	out := captureStdout(t, func() {
		cmdCheck(repo, []string{"bet the whole treasury", "--reward", "0.95", "--signal", "ruin_risk=1", "--signal", "unrepeated=0", "--json"})
	})
	if !strings.Contains(out, `"allowed": false`) || !strings.Contains(out, "never-ruin") {
		t.Fatalf("hard-constraint veto must fire regardless of conviction dormancy; got:\n%s", out)
	}
}

// TestInstallSkillsHelpNoSideEffect: `install-skills --help` must print usage and
// write nothing — a help flag that mutates the filesystem was the observed bug.
func TestInstallSkillsHelpNoSideEffect(t *testing.T) {
	skillDir := filepath.Join(t.TempDir(), "skills-target")

	out := captureStdout(t, func() {
		cmdInstallSkills([]string{"--skill-dir", skillDir, "--help"})
	})
	if !strings.Contains(out, "usage: brain install-skills") {
		t.Fatalf("--help should print usage; got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); !os.IsNotExist(err) {
		t.Fatalf("--help must not install anything, but SKILL.md exists (err=%v)", err)
	}
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("--help must not create the skill dir, but it exists")
	}
}

package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// captureStdout redirects os.Stdout for the duration of fn and returns
// everything written to it. cmdCheck (via emit) writes its --json output
// straight to os.Stdout, so this is how a CLI-level test observes it.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	orig := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("close pipe writer: %v", err)
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("read pipe: %v", err)
	}
	return buf.String()
}

// TestCmdCheckUndeterminedWhenSignalOmitted is the CLI-level regression test
// for the bug described in docs/SPEC-shield-signal-provenance-v1.md section 1:
// `brain check "bet the account" --reward 0.95 --json` with the constraints'
// named signals NOT passed used to silently report allowed:true,
// guaranteed:true. cmdInit's starter constraints.json declares both
// never-ruin (signal ruin_risk) and never-n1 (signal unrepeated) as Hard
// constraints with no when_absent, so both rely on the default fail-closed
// ("veto") policy: omitting their signals must now yield allowed:false,
// undetermined:true, undetermined_by:["never-n1","never-ruin"] (sorted).
func TestCmdCheckUndeterminedWhenSignalOmitted(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)

	out := captureStdout(t, func() {
		cmdCheck(repo, []string{"bet the account", "--reward", "0.95", "--json"})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal check --json output: %v\noutput: %s", err, out)
	}

	if allowed, _ := got["allowed"].(bool); allowed {
		t.Errorf("allowed = %v, want false (signals were never supplied)", got["allowed"])
	}
	if undetermined, _ := got["undetermined"].(bool); !undetermined {
		t.Errorf("undetermined = %v, want true (signals were never supplied)", got["undetermined"])
	}
	by, ok := got["undetermined_by"].([]any)
	if !ok || len(by) != 2 || by[0] != "never-n1" || by[1] != "never-ruin" {
		t.Errorf("undetermined_by = %v, want [\"never-n1\",\"never-ruin\"]", got["undetermined_by"])
	}
	if guaranteed, _ := got["guaranteed"].(bool); guaranteed {
		t.Errorf("guaranteed = %v, want false", got["guaranteed"])
	}
	if vetoed, _ := got["vetoed_by"].([]any); len(vetoed) != 0 {
		t.Errorf("vetoed_by = %v, want empty (nothing was actually measured as violating)", got["vetoed_by"])
	}
}

// TestCmdCheckAllowedWhenSignalSupplied is the companion happy-path case:
// supplying ruin_risk within threshold must still allow the decision, proving
// the fail-closed fix distinguishes "provided and safe" from "never provided".
func TestCmdCheckAllowedWhenSignalSupplied(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)

	out := captureStdout(t, func() {
		cmdCheck(repo, []string{"a measured decision", "--reward", "0.2", "--signal", "ruin_risk=0", "--signal", "unrepeated=0", "--json"})
	})

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal check --json output: %v\noutput: %s", err, out)
	}

	if allowed, _ := got["allowed"].(bool); !allowed {
		t.Errorf("allowed = %v, want true (signals supplied and within threshold)", got["allowed"])
	}
	if undetermined, _ := got["undetermined"].(bool); undetermined {
		t.Errorf("undetermined = %v, want false", got["undetermined"])
	}
	if guaranteed, _ := got["guaranteed"].(bool); !guaranteed {
		t.Errorf("guaranteed = %v, want true (every constraint's cost was actually measured)", got["guaranteed"])
	}
}

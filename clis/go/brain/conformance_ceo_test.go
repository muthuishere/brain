package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/muthuishere/brain/libs/go/engine"
)

// mustStore opens the on-disk brain at repo as a FileStore, for asserting
// episode counts directly (the same store the CLI writes through).
func mustStore(t *testing.T, repo string) *engine.FileStore {
	t.Helper()
	fs, err := engine.NewFileStore(repo)
	if err != nil {
		t.Fatalf("NewFileStore(%s): %v", repo, err)
	}
	return fs
}

// TestConformanceCEODecisionLoop pins the loop the CEO operates the brain on:
// recall-before-decision, check-before-action, record-after-outcome. It is the
// brain organ's conformance contract for the CEO use case — if any leg regresses,
// this fails. Driven entirely over the CLI surface (package main), offline.
func TestConformanceCEODecisionLoop(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)
	cmdObjective(repo, []string{"drive one paying customer per day"})

	// --- record-after-outcome: two consistent episodes about the same lesson ---
	cmdRecord(repo, []string{"published a page claiming it was real over fabricated data — a fake", "--reward", "-0.9"})
	cmdRecord(repo, []string{"faked a real claim again without a citation to back it", "--reward", "-0.85"})
	captureStdout(t, func() { cmdConsolidate(repo, nil) })

	// --- recall-before-decision: the lesson comes back grounded, with citations ---
	rec := captureStdout(t, func() {
		cmdRecall(repo, []string{"claim something is real", "--json"})
	})
	var r map[string]any
	if err := json.Unmarshal([]byte(rec), &r); err != nil {
		t.Fatalf("recall --json: %v\n%s", err, rec)
	}
	if grounded, _ := r["grounded"].(bool); !grounded {
		t.Fatalf("recall must be grounded in prior experience; got: %s", rec)
	}
	if sup, _ := r["supporting"].(float64); sup < 1 {
		t.Fatalf("recall must cite supporting episodes; got supporting=%v", r["supporting"])
	}

	// --- check-before-action: a ruinous decision is vetoed (profitable-but-ruinous) ---
	veto := captureStdout(t, func() {
		cmdCheck(repo, []string{"bet the company on one launch", "--reward", "0.95", "--signal", "ruin_risk=1", "--signal", "unrepeated=0", "--json"})
	})
	var v map[string]any
	if err := json.Unmarshal([]byte(veto), &v); err != nil {
		t.Fatalf("check --json: %v\n%s", err, veto)
	}
	if allowed, _ := v["allowed"].(bool); allowed {
		t.Fatalf("ruinous decision must be vetoed; got: %s", veto)
	}
	if alarm, _ := v["alarm"].(bool); !alarm {
		t.Fatalf("high-reward + violated constraint must raise the profitable-but-ruinous alarm; got: %s", veto)
	}

	// --- check-before-action: fail-closed when a hard constraint's signal is absent ---
	fc := captureStdout(t, func() {
		cmdCheck(repo, []string{"act without measuring risk", "--reward", "0.9", "--json"})
	})
	var f map[string]any
	if err := json.Unmarshal([]byte(fc), &f); err != nil {
		t.Fatalf("check --json: %v\n%s", err, fc)
	}
	if allowed, _ := f["allowed"].(bool); allowed {
		t.Fatalf("absent hard-constraint signal must fail closed (not allowed); got: %s", fc)
	}
	if undet, _ := f["undetermined"].(bool); !undet {
		t.Fatalf("absent hard-constraint signal must be undetermined; got: %s", fc)
	}
	if guar, _ := f["guaranteed"].(bool); guar {
		t.Fatalf("nothing can be guaranteed when a signal was never measured; got: %s", fc)
	}

	// --- record-after-outcome persists a new, retrievable episode ---
	fs := mustStore(t, repo)
	before := len(fs.Episodes())
	cmdRecord(repo, []string{"a fresh decision with its realized outcome", "--reward", "0.4"})
	after := len(mustStore(t, repo).Episodes())
	if after != before+1 {
		t.Fatalf("record-after-outcome must append exactly one episode; before=%d after=%d", before, after)
	}
}

// TestConformanceCrossOrganWriteIn documents+pins how OTHER organs use the brain:
// (1) citenexus verdicts are recorded as episodes and are later recallable, so the
// brain accumulates what citenexus verified; (2) a fleet worker can load the
// playbook (DO/AVOID rules) before acting.
func TestConformanceCrossOrganWriteIn(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)
	cmdObjective(repo, []string{"verifiable autonomy"})

	// (1) citenexus writes a verdict in as an episode.
	cmdRecord(repo, []string{"citenexus ABSTAIN: 'homepage live' claim had no cited 200 response — refused to confirm", "--reward", "0.7", "--tag", "citenexus"})
	cmdRecord(repo, []string{"citenexus CONFIRM: signup row present in DB with a cited query — claim verified", "--reward", "0.8", "--tag", "citenexus"})

	got := captureStdout(t, func() {
		cmdRecall(repo, []string{"did citenexus verify the homepage claim", "--json"})
	})
	var r map[string]any
	if err := json.Unmarshal([]byte(got), &r); err != nil {
		t.Fatalf("recall --json: %v\n%s", err, got)
	}
	if grounded, _ := r["grounded"].(bool); !grounded {
		t.Fatalf("a recorded citenexus verdict must be recallable; got: %s", got)
	}

	// (2) fleet worker loads the playbook before acting.
	cmdRecord(repo, []string{"loaded playbook before acting and avoided a known failure path", "--reward", "0.8"})
	cmdRecord(repo, []string{"loaded playbook again before acting and it steered the plan", "--reward", "0.75"})
	captureStdout(t, func() { cmdConsolidate(repo, nil) })
	captureStdout(t, func() { cmdReflect(repo, nil) })
	captureStdout(t, func() { cmdCurate(repo, []string{"--apply"}) })

	pb := captureStdout(t, func() { cmdPlaybook(repo, nil) })
	if strings.TrimSpace(pb) == "" || !(strings.Contains(pb, "DO") || strings.Contains(pb, "AVOID")) {
		t.Fatalf("playbook must emit loadable DO/AVOID rules for a worker; got:\n%s", pb)
	}
}

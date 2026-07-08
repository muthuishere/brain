// The self-evolving verbs (Phase A): reflect → regress-gate → curate, over
// git-friendly files in the brain repo:
//
//	playbook.json                the itemized playbook agents load pre-act
//	pending-deltas.ndjson        proposed deltas awaiting curation (append-only)
//	rejected-candidates.ndjson   gate rejections — logged, never applied
//	evolve-policy.json           acceptance thresholds (owner-edited, not in-run)
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/muthuishere/brain/libs/go/engine"
)

func loadEvolvePolicy(repo string) engine.EvolvePolicy {
	p := engine.DefaultEvolvePolicy()
	data, err := os.ReadFile(filepath.Join(repo, "evolve-policy.json"))
	if err != nil {
		return p
	}
	if err := json.Unmarshal(data, &p); err != nil {
		fatal("evolve-policy.json: %v", err)
	}
	return p
}

func loadPlaybook(repo string) engine.Playbook {
	var pb engine.Playbook
	data, err := os.ReadFile(filepath.Join(repo, "playbook.json"))
	if err != nil {
		return pb
	}
	if err := json.Unmarshal(data, &pb); err != nil {
		fatal("playbook.json: %v", err)
	}
	return pb
}

func savePlaybook(repo string, pb engine.Playbook) {
	data, _ := json.MarshalIndent(pb, "", "  ")
	if err := os.WriteFile(filepath.Join(repo, "playbook.json"), data, 0o644); err != nil {
		fatal("playbook.json: %v", err)
	}
}

func loadDeltas(repo, name string) []engine.Delta {
	f, err := os.Open(filepath.Join(repo, name))
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []engine.Delta
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var d engine.Delta
		if err := json.Unmarshal(sc.Bytes(), &d); err != nil {
			fatal("%s: %v", name, err)
		}
		out = append(out, d)
	}
	return out
}

func appendDeltas(repo, name string, deltas []engine.Delta) {
	if len(deltas) == 0 {
		return
	}
	f, err := os.OpenFile(filepath.Join(repo, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		fatal("%s: %v", name, err)
	}
	defer f.Close()
	for _, d := range deltas {
		line, _ := json.Marshal(d)
		_, _ = f.Write(append(line, '\n'))
	}
}

func writeDeltas(repo, name string, deltas []engine.Delta) {
	f, err := os.Create(filepath.Join(repo, name))
	if err != nil {
		fatal("%s: %v", name, err)
	}
	defer f.Close()
	w := bufio.NewWriter(f)
	for _, d := range deltas {
		line, _ := json.Marshal(d)
		_, _ = w.Write(append(line, '\n'))
	}
	_ = w.Flush()
}

// cmdReflect distills recent episodes into candidate deltas and queues the new
// ones in pending-deltas.ndjson. It never touches the playbook.
func cmdReflect(repo string, args []string) {
	f := parseFlags(args)
	since := 0.0
	if v, ok := f.floats["since"]; ok {
		since = v
	}
	b := openBrain(repo)
	policy := loadEvolvePolicy(repo)
	deltas := engine.Reflect(b.Namespace(), b.Episodes(), since, policy, now())
	known := map[string]bool{}
	for _, d := range loadDeltas(repo, "pending-deltas.ndjson") {
		known[d.ID] = true
	}
	for _, d := range loadDeltas(repo, "rejected-candidates.ndjson") {
		known[d.ID] = true
	}
	for _, e := range loadPlaybook(repo).Entries {
		for _, id := range e.SourceDeltas {
			known[id] = true
		}
	}
	var fresh []engine.Delta
	for _, d := range deltas {
		if !known[d.ID] {
			fresh = append(fresh, d)
		}
	}
	appendDeltas(repo, "pending-deltas.ndjson", fresh)
	if f.bools["json"] {
		emit(map[string]any{"proposed": deltas, "queued_new": len(fresh)})
		return
	}
	fmt.Printf("deltas proposed: %d  newly queued: %d\n", len(deltas), len(fresh))
	for _, d := range fresh {
		fmt.Printf("- %s [%s] %s (n=%d, consistency %.2f)\n", d.ID, d.Valence, d.Rule, len(d.EvidenceIDs), d.Consistency)
	}
}

// cmdCurate gates every pending delta (regress) and merges the survivors into
// the playbook. Dry-run by default; --apply persists playbook + queues.
func cmdCurate(repo string, args []string) {
	f := parseFlags(args)
	runCurate(repo, f.bools["apply"], f.bools["json"])
}

func runCurate(repo string, apply, jsonOut bool) {
	b := openBrain(repo)
	policy := loadEvolvePolicy(repo)
	pending := loadDeltas(repo, "pending-deltas.ndjson")
	convictions := b.Convictions(false)
	var accepted, rejected []engine.Delta
	for _, d := range pending {
		res := engine.Regress(d, convictions, policy)
		if res.Passed {
			d.Status = "applied"
			accepted = append(accepted, d)
		} else {
			d.Status = "rejected"
			d.Reason = res.Reason
			rejected = append(rejected, d)
		}
	}
	pb := loadPlaybook(repo)
	rep := engine.Curate(b.Namespace(), &pb, accepted, policy, now())
	if apply {
		savePlaybook(repo, pb)
		appendDeltas(repo, "rejected-candidates.ndjson", rejected)
		writeDeltas(repo, "pending-deltas.ndjson", nil)
	}
	if jsonOut {
		emit(map[string]any{"applied": apply, "accepted": len(accepted), "rejected": rejected, "report": rep})
		return
	}
	mode := "dry-run (use --apply to persist)"
	if apply {
		mode = "applied"
	}
	fmt.Printf("%s — accepted: %d  rejected: %d  merged: %d  added: %d  superseded: %d  active rules: %d\n",
		mode, len(accepted), len(rejected), rep.Merged, rep.Added, rep.Superseded, rep.Total)
	for _, d := range rejected {
		fmt.Printf("  rejected %s: %s\n", d.ID, d.Reason)
	}
}

// cmdPlaybook prints the itemized playbook an agent loads before acting.
func cmdPlaybook(repo string, args []string) {
	f := parseFlags(args)
	pb := loadPlaybook(repo)
	entries := pb.TopicEntries(f.strs["topic"])
	if f.bools["json"] {
		emit(entries)
		return
	}
	if len(entries) == 0 {
		fmt.Println("(playbook empty — record experiences, then reflect + curate --apply)")
		return
	}
	for _, e := range entries {
		fmt.Printf("- [%s] %s (n=%d, reward %.2f)\n", e.Valence, e.Rule, e.SupportCount, e.Reward)
	}
}

// cmdRegress runs the gate on one pending delta without applying anything.
func cmdRegress(repo string, args []string) {
	id, f := firstArgAndFlags(args)
	if id == "" {
		fatal("regress needs a delta id")
	}
	b := openBrain(repo)
	policy := loadEvolvePolicy(repo)
	for _, d := range loadDeltas(repo, "pending-deltas.ndjson") {
		if d.ID != id {
			continue
		}
		res := engine.Regress(d, b.Convictions(false), policy)
		if f.bools["json"] {
			emit(res)
			return
		}
		fmt.Printf("passed: %v — %s\n", res.Passed, res.Reason)
		return
	}
	fatal("unknown pending delta: %s", id)
}

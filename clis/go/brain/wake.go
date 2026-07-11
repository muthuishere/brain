package main

// wake.go — the orientation contract (CoALA "retrieval into working memory").
//
// One read at session start / loop tick: it renders CHARTER (objective + top
// convictions), RISK (the constraint envelope) and STATE (the live board). STATE
// values that are stale or unknown are flagged, never rendered as confident-current
// — and if a desk heartbeat is stale/absent the whole STATE block is marked
// suspect, so a lapsed writer degrades to "probably down", not a false "running".
// v1 is state-focused; stable FACTS stay in the project's CLAUDE.md for now.
// See docs/SPEC-orientation-layer-v1.md.

import (
	"fmt"
	"sort"
	"strings"
)

func cmdWake(repo string, args []string) { wakeMain(repo, args, now()) }

func wakeMain(repo string, args []string, nowTs float64) {
	f := parseFlags(args)
	desk := f.strs["as"]
	b := openBrain(repo)

	// CHARTER
	objective := ""
	if o, ok := b.Objective(); ok {
		objective = o.Text
	}
	convs := b.Convictions(false)
	sort.Slice(convs, func(i, j int) bool { return convs[i].Confidence > convs[j].Confidence })
	if len(convs) > 5 {
		convs = convs[:5]
	}

	// RISK (envelope summary; not a live check)
	constraints := loadConstraints(repo)

	// STATE
	st := loadState(repo)
	keys := make([]string, 0, len(st))
	for k := range st {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	staleOrUnknownHeartbeat := false
	anyHeartbeat := false
	for k, e := range st {
		if strings.HasPrefix(k, "heartbeat.") {
			anyHeartbeat = true
			if s, _ := stateStatus(e, nowTs); s == "stale" {
				staleOrUnknownHeartbeat = true
			}
		}
	}

	if f.bools["json"] {
		type kv struct {
			Key, Value, Status string
			AgeSec, TTLSec     float64
		}
		var state []kv
		for _, k := range keys {
			s, age := stateStatus(st[k], nowTs)
			state = append(state, kv{k, st[k].Value, s, age, st[k].TTLSec})
		}
		emit(map[string]any{
			"desk":              desk,
			"objective":         objective,
			"convictions":       convs,
			"constraints":       constraints,
			"state":             state,
			"heartbeat_suspect": staleOrUnknownHeartbeat || !anyHeartbeat,
		})
		return
	}

	label := desk
	if label == "" {
		label = "(no desk)"
	}
	fmt.Printf("=== brain wake --as %s ===\n", label)

	fmt.Println("\n[CHARTER]")
	if objective != "" {
		fmt.Printf("- objective: %s\n", objective)
	} else {
		fmt.Println("- objective: (none set)")
	}
	for _, cv := range convs {
		fmt.Printf("- [%s] %s (n=%d, conf %.2f)\n", cv.Valence, cv.Statement, cv.SupportCount, cv.Confidence)
	}

	fmt.Println("\n[RISK envelope]")
	if len(constraints) == 0 {
		fmt.Println("- (no constraints.json)")
	}
	for _, c := range constraints {
		fmt.Printf("- %s: %s\n", c.Name, c.Text)
	}

	fmt.Println("\n[STATE]")
	if len(keys) == 0 {
		fmt.Println("- (no state recorded)")
	}
	if staleOrUnknownHeartbeat || (!anyHeartbeat && len(keys) > 0) {
		fmt.Println("- ⚠ heartbeat stale/absent — the loop is probably DOWN; treat all STATE below as suspect, do NOT claim it's running")
	}
	for _, k := range keys {
		s, age := stateStatus(st[k], nowTs)
		fmt.Printf("- %s = %s  [%s, %s ago]\n", k, st[k].Value, strings.ToUpper(s), humanAge(age))
	}
	fmt.Println("\n[FACTS] (v1: stable facts live in the project CLAUDE.md; fact layer deferred)")
}

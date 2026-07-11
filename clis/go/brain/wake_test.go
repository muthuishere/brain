package main

import (
	"strings"
	"testing"
)

// wake must, when a desk heartbeat is stale, mark the whole STATE block suspect
// so the agent reports "probably down" instead of confabulating "running".
func TestWakeFlagsStaleHeartbeat(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)
	cmdObjective(repo, []string{"F&O intraday only; never idle"})
	stateMain(repo, []string{"set", "heartbeat.fo", "alive", "--ttl", "300s"}, 1000)
	stateMain(repo, []string{"set", "positions.fo", "0 open", "--ttl", "12h"}, 1000)

	// t=2000: heartbeat age 1000 > 300 -> stale -> STATE block must be flagged suspect
	out := captureStdout(t, func() { wakeMain(repo, []string{"--as", "fo"}, 2000) })
	if !strings.Contains(out, "[CHARTER]") || !strings.Contains(out, "never idle") {
		t.Fatalf("wake missing charter/objective:\n%s", out)
	}
	if !strings.Contains(strings.ToLower(out), "probably down") {
		t.Fatalf("stale heartbeat must mark STATE suspect (probably down):\n%s", out)
	}
	if !strings.Contains(out, "STALE") {
		t.Fatalf("stale heartbeat must render STALE:\n%s", out)
	}
}

// With a fresh heartbeat, wake must NOT cry "probably down".
func TestWakeFreshHeartbeatNotSuspect(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)
	stateMain(repo, []string{"set", "heartbeat.fo", "alive", "--ttl", "300s"}, 1000)
	out := captureStdout(t, func() { wakeMain(repo, []string{"--as", "fo"}, 1100) }) // age 100 <= 300
	if strings.Contains(strings.ToLower(out), "probably down") {
		t.Fatalf("fresh heartbeat should not be flagged down:\n%s", out)
	}
	if !strings.Contains(out, "FRESH") {
		t.Fatalf("fresh heartbeat should render FRESH:\n%s", out)
	}
}

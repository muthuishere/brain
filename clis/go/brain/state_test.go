package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// The state layer's whole point is that a value past its TTL is reported STALE,
// never FRESH — the fail-safe that stops "is it running?" being answered with a
// hallucinated yes after the writer dies. Time is injected so this is deterministic.

func TestStateFreshThenStale(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)

	// set heartbeat at t=1000 with a 300s TTL
	stateMain(repo, []string{"set", "heartbeat.fo", "alive", "--ttl", "300s"}, 1000)

	// t=1200: age 200 <= 300 -> FRESH
	out := captureStdout(t, func() { stateMain(repo, []string{"get", "heartbeat.fo", "--json"}, 1200) })
	var g map[string]any
	if err := json.Unmarshal([]byte(out), &g); err != nil {
		t.Fatalf("json: %v (%s)", err, out)
	}
	if g["status"] != "fresh" {
		t.Fatalf("t=1200 want fresh, got %v", g["status"])
	}

	// t=1400: age 400 > 300 -> STALE (must NOT be fresh)
	out = captureStdout(t, func() { stateMain(repo, []string{"get", "heartbeat.fo", "--json"}, 1400) })
	json.Unmarshal([]byte(out), &g)
	if g["status"] != "stale" {
		t.Fatalf("t=1400 want stale, got %v — a stale value served as fresh is the dangerous bug", g["status"])
	}
}

func TestStateUnknownAndStatic(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)

	out := captureStdout(t, func() { stateMain(repo, []string{"get", "never.set", "--json"}, 500) })
	if !strings.Contains(out, `"status": "unknown"`) {
		t.Fatalf("unknown key must report unknown, got %s", out)
	}

	// --static: a value with no TTL never goes stale
	stateMain(repo, []string{"set", "last_ritual_date", "2026-07-11", "--static"}, 1000)
	out = captureStdout(t, func() { stateMain(repo, []string{"get", "last_ritual_date", "--json"}, 999999) })
	var g map[string]any
	json.Unmarshal([]byte(out), &g)
	if g["status"] != "static" {
		t.Fatalf("static value should stay static, got %v", g["status"])
	}
}

func TestStateNegativeValueAccepted(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)
	// P&L is often negative; the value must not be parsed as a flag.
	stateMain(repo, []string{"set", "pnl.fo.today", "-4200", "--ttl", "12h"}, 1000)
	out := captureStdout(t, func() { stateMain(repo, []string{"get", "pnl.fo.today", "--json"}, 1000) })
	var g map[string]any
	json.Unmarshal([]byte(out), &g)
	if g["value"] != "-4200" {
		t.Fatalf("negative value round-trip failed, got %v", g["value"])
	}
}

func TestStateListStaleFilter(t *testing.T) {
	repo := t.TempDir()
	cmdInit(repo)
	stateMain(repo, []string{"set", "heartbeat.fo", "alive", "--ttl", "300s"}, 1000) // will be stale at 2000
	stateMain(repo, []string{"set", "pnl.fo.today", "-100", "--ttl", "24h"}, 1000)   // still fresh at 2000
	out := captureStdout(t, func() { stateMain(repo, []string{"list", "--stale"}, 2000) })
	if !strings.Contains(out, "heartbeat.fo") || strings.Contains(out, "pnl.fo.today") {
		t.Fatalf("--stale should list only the stale heartbeat, got:\n%s", out)
	}
}

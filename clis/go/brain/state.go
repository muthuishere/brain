package main

// state.go — the volatile working-memory layer (CoALA "working memory").
//
// state.ndjson is an append-only log; the latest record per key wins. Every
// value carries a TTL: reads report fresh|stale|unknown|static and the age, and
// a stale/unknown value is NEVER rendered as a confident current value. This is
// the fail-safe that stops "is it running?" from being answered with a
// hallucinated yes when the writer has died. See docs/SPEC-orientation-layer-v1.md.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// StateEntry is one datapoint of volatile desk state.
type StateEntry struct {
	Key    string  `json:"key"`
	Value  string  `json:"value"`
	Ts     float64 `json:"ts"`
	TTLSec float64 `json:"ttl_sec"`
	Source string  `json:"source,omitempty"`
}

func statePath(repo string) string { return filepath.Join(repo, "state.ndjson") }

// loadState reads state.ndjson and keeps the latest record per key.
func loadState(repo string) map[string]StateEntry {
	out := map[string]StateEntry{}
	f, err := os.Open(statePath(repo))
	if err != nil {
		return out // missing file is fine — no state yet
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e StateEntry
		if json.Unmarshal([]byte(line), &e) == nil && e.Key != "" {
			out[e.Key] = e // latest wins
		}
	}
	return out
}

func appendState(repo string, e StateEntry) error {
	f, err := os.OpenFile(statePath(repo), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	line, _ := json.Marshal(e)
	_, err = f.Write(append(line, '\n'))
	return err
}

// stateStatus classifies an entry at time now.
//   - TTLSec <= 0  -> "static"  (a value with no expiry; e.g. last_ritual_date)
//   - age <= TTL   -> "fresh"
//   - age >  TTL   -> "stale"
func stateStatus(e StateEntry, now float64) (status string, age float64) {
	age = now - e.Ts
	if e.TTLSec <= 0 {
		return "static", age
	}
	if age <= e.TTLSec {
		return "fresh", age
	}
	return "stale", age
}

func humanAge(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	switch {
	case sec < 90:
		return fmt.Sprintf("%.0fs", sec)
	case sec < 5400:
		return fmt.Sprintf("%.0fm", sec/60)
	case sec < 172800:
		return fmt.Sprintf("%.1fh", sec/3600)
	default:
		return fmt.Sprintf("%.1fd", sec/86400)
	}
}

func parseTTL(s string) (float64, bool) {
	if s == "" {
		return 0, false
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, false
	}
	return d.Seconds(), true
}

// leadingPos splits up to n leading positional args from the flags that follow.
func leadingPos(args []string, n int) ([]string, flags) {
	i := 0
	for i < len(args) && i < n && !strings.HasPrefix(args[i], "-") {
		i++
	}
	return args[:i], parseFlags(args[i:])
}

func cmdState(repo string, args []string) { stateMain(repo, args, now()) }

// stateMain is the testable core (clock injected).
func stateMain(repo string, args []string, nowTs float64) {
	if len(args) == 0 {
		fatal("state needs a subcommand: set | get | list | heartbeat")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "set":
		// key and value are the first two tokens, positionally — value may start
		// with '-' (negative P&L), so do NOT treat it as a flag. Flags follow.
		if len(rest) < 2 {
			fatal("state set <key> <value> [--ttl DUR | --static] [--source WHO]")
		}
		key, value := rest[0], rest[1]
		f := parseFlags(rest[2:])
		ttl, hasTTL := parseTTL(f.strs["ttl"])
		if !hasTTL && !f.bools["static"] {
			// Fail-safe: forbid the silent no-TTL default. A value with no
			// freshness bound would be served as current forever after its
			// writer dies. Force an explicit choice.
			fatal("state set requires --ttl DUR (e.g. 5m, 900s) or explicit --static for a non-expiring value")
		}
		e := StateEntry{Key: key, Value: value, Ts: nowTs, TTLSec: ttl, Source: f.strs["source"]}
		if err := appendState(repo, e); err != nil {
			fatal("state set: %v", err)
		}
		if f.bools["json"] {
			emit(e)
		} else {
			fmt.Printf("set %s = %s\n", e.Key, e.Value)
		}
	case "heartbeat":
		pos, f := leadingPos(rest, 1)
		if len(pos) < 1 {
			fatal("state heartbeat <name> [--ttl DUR] [--note N]")
		}
		ttl, ok := parseTTL(f.strs["ttl"])
		if !ok {
			ttl = 300 // default 5m heartbeat window
		}
		val := "alive"
		if n := f.strs["note"]; n != "" {
			val = "alive: " + n
		}
		e := StateEntry{Key: "heartbeat." + pos[0], Value: val, Ts: nowTs, TTLSec: ttl, Source: f.strs["source"]}
		if err := appendState(repo, e); err != nil {
			fatal("heartbeat: %v", err)
		}
		fmt.Printf("heartbeat %s (ttl %s)\n", pos[0], humanAge(ttl))
	case "get":
		pos, f := leadingPos(rest, 1)
		if len(pos) < 1 {
			fatal("state get <key>")
		}
		st := loadState(repo)
		e, ok := st[pos[0]]
		if !ok {
			if f.bools["json"] {
				emit(map[string]any{"key": pos[0], "status": "unknown", "value": nil})
			} else {
				fmt.Printf("%s: UNKNOWN — never recorded\n", pos[0])
			}
			return
		}
		status, age := stateStatus(e, nowTs)
		if f.bools["json"] {
			emit(map[string]any{"key": e.Key, "value": e.Value, "status": status,
				"age_sec": age, "ttl_sec": e.TTLSec, "source": e.Source})
			return
		}
		fmt.Printf("%s = %s  [%s, %s ago]\n", e.Key, e.Value, strings.ToUpper(status), humanAge(age))
	case "list":
		_, f := leadingPos(rest, 0)
		st := loadState(repo)
		type row struct {
			Key, Value, Status string
			Age, TTL           float64
		}
		var rows []row
		for _, e := range st {
			status, age := stateStatus(e, nowTs)
			if f.bools["stale"] && status == "fresh" {
				continue
			}
			if f.bools["stale"] && status == "static" {
				continue
			}
			rows = append(rows, row{e.Key, e.Value, status, age, e.TTLSec})
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].Key < rows[j].Key })
		if f.bools["json"] {
			emit(rows)
			return
		}
		if len(rows) == 0 {
			fmt.Println("(no state)")
			return
		}
		for _, r := range rows {
			fmt.Printf("%-28s %-24s [%s, %s ago]\n", r.Key, r.Value, strings.ToUpper(r.Status), humanAge(r.Age))
		}
	default:
		fatal("state: unknown subcommand %q (set|get|list|heartbeat)", sub)
	}
}

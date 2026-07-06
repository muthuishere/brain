// Command brain is a standalone, dependency-light CLI for the CiteNexus Brain.
//
// It is deliberately dumb: you point it at a folder ("the repo") and it records,
// recalls, checks decisions against constraints, and consolidates — all
// deterministic, no network, no model endpoint. The agent skill on top handles
// which repo, git commits, and multiple brains; this binary just does the work.
//
//	brain --repo ./mybrain init
//	brain --repo ./mybrain objective "preserve capital"
//	brain --repo ./mybrain record "Overtrading in chop bled the account" --reward -1
//	brain --repo ./mybrain recall "overtrading" --json
//	brain --repo ./mybrain check "bet the account" --reward 0.95 --signal ruin_risk=1 --json
//	brain --repo ./mybrain consolidate
package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/muthuishere/brain/libs/go/engine"
	"github.com/muthuishere/brain/libs/go/ingest"
)

//go:embed skill/SKILL.md
var embeddedSkill []byte

//go:embed skill/config.example.json
var embeddedConfig []byte

//go:embed skill/endpoints.example.json
var embeddedEndpoints []byte

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	// Global --repo (or $BRAIN_REPO, or cwd) is stripped from args first.
	repo, rest := extractRepo(os.Args[1:])
	if len(rest) == 0 {
		usage()
		os.Exit(2)
	}
	cmd, args := rest[0], rest[1:]

	switch cmd {
	case "init":
		cmdInit(repo)
	case "objective":
		cmdObjective(repo, args)
	case "record":
		cmdRecord(repo, args)
	case "reappraise":
		cmdReappraise(repo, args)
	case "recall":
		cmdRecall(repo, args)
	case "learn":
		cmdLearn(repo, args)
	case "check":
		cmdCheck(repo, args)
	case "consolidate":
		cmdConsolidate(repo, args)
	case "convictions":
		cmdConvictions(repo, args)
	case "status":
		cmdStatus(repo)
	case "install-skills":
		cmdInstallSkills(args)
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "brain: unknown command %q\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `brain — CiteNexus Brain CLI (deterministic; point it at a repo folder)

usage: brain [--repo DIR] <command> [args]

commands:
  init                              create the brain folder + starter constraints.json
  objective ["TEXT"]                set the foreground objective (or print it)
  record "TEXT" [--reward R] [--label L] [--dimension D]
  record --from-file PATH [--max-tokens N] [--overlap N] [--reward R] [--label L] [--dimension D] [--json]
                                     chunk a local file (ingest.ChunkFile) and record one episode per
                                     chunk; --max-tokens/--overlap default to 450/60 (chunker defaults);
                                     TEXT and --from-file are mutually exclusive
  reappraise ID --reward R [--label L] [--note N]
  recall "QUERY" [-k N] [--json]    grounded, cite-or-abstain recall
  learn "TOPIC" [--json]            what validated convictions cover a topic
  check "DECISION" --reward R [--signal name=val ...] [--fallback F] [--json]
  consolidate [--min-support N] [--min-consistency C] [--forget] [--json]
  convictions [--json]             the brain's current point of view
  status                           objective + counts
  install-skills                   install the agent skill + seed ~/.config/brain/config.json

repo: --repo DIR, else $BRAIN_REPO, else current directory.
`)
}

// --- repo + brain construction ---------------------------------------------

func extractRepo(args []string) (string, []string) {
	repo := os.Getenv("BRAIN_REPO")
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--repo" && i+1 < len(args) {
			repo = args[i+1]
			i++
			continue
		}
		if strings.HasPrefix(args[i], "--repo=") {
			repo = strings.TrimPrefix(args[i], "--repo=")
			continue
		}
		out = append(out, args[i])
	}
	if repo == "" {
		repo, _ = os.Getwd()
	}
	return repo, out
}

func now() float64 { return float64(time.Now().Unix()) }

func openBrain(repo string) *engine.Brain {
	fs, err := engine.NewFileStore(repo)
	if err != nil {
		fatal("open brain: %v", err)
	}
	ns := filepath.Base(strings.TrimRight(repo, "/"))
	if ns == "" || ns == "." {
		ns = "default"
	}
	// Endpoints are opt-in (endpoints.json / env). With none, this is the offline
	// deterministic path: hashing embedder, no reranker, no LLM — nothing to run.
	embedder, llm, reranker := buildModels(loadEndpoints(repo))
	b := engine.NewWithStore(embedder, llm, ns, loadConstraints(repo), 0.5, fs, now)
	if reranker != nil {
		b.SetReranker(reranker)
	}
	return b
}

// --- constraints (declarative, serializable) --------------------------------

type constraintFile struct {
	Name      string  `json:"name"`
	Text      string  `json:"text"`
	Kind      string  `json:"kind"`
	Signal    string  `json:"signal"`
	Threshold float64 `json:"threshold"`
	Weight    float64 `json:"weight"`
}

func loadConstraints(repo string) []engine.Constraint {
	data, err := os.ReadFile(filepath.Join(repo, "constraints.json"))
	if err != nil {
		return nil
	}
	var raw []constraintFile
	if err := json.Unmarshal(data, &raw); err != nil {
		fatal("constraints.json: %v", err)
	}
	out := make([]engine.Constraint, 0, len(raw))
	for _, c := range raw {
		kind := engine.Hard
		if strings.EqualFold(c.Kind, "soft") {
			kind = engine.Soft
		}
		w := c.Weight
		if w == 0 {
			w = 1
		}
		out = append(out, engine.Constraint{
			Name: c.Name, Text: c.Text, Kind: kind,
			Threshold: c.Threshold, Weight: w, Signal: c.Signal,
		})
	}
	return out
}

// --- commands ---------------------------------------------------------------

func cmdInit(repo string) {
	if err := os.MkdirAll(repo, 0o755); err != nil {
		fatal("init: %v", err)
	}
	path := filepath.Join(repo, "constraints.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		starter := []constraintFile{
			{Name: "never-ruin", Text: "never risk ruin / irrecoverable loss", Kind: "hard", Signal: "ruin_risk"},
			{Name: "never-n1", Text: "never act on a single unrepeated result", Kind: "hard", Signal: "unrepeated"},
		}
		data, _ := json.MarshalIndent(starter, "", "  ")
		_ = os.WriteFile(path, data, 0o644)
	}
	// Drop an endpoints example (offline by default — rename to endpoints.json to
	// enable a real embedding / reranker / LLM). The brain works without it.
	exPath := filepath.Join(repo, "endpoints.example.json")
	if _, err := os.Stat(exPath); os.IsNotExist(err) {
		_ = os.WriteFile(exPath, embeddedEndpoints, 0o644)
	}
	// Touch the store so the folder is a valid brain immediately.
	openBrain(repo)
	fmt.Printf("initialized brain at %s\n", repo)
}

func cmdObjective(repo string, args []string) {
	b := openBrain(repo)
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		if obj, ok := b.Objective(); ok {
			fmt.Println(obj.Text)
		} else {
			fmt.Println("(no objective set)")
		}
		return
	}
	obj := b.SetObjective(args[0], "")
	fmt.Printf("objective set: %s\n", obj.Text)
}

func cmdRecord(repo string, args []string) {
	text, f := firstArgAndFlags(args)
	if path, hasFile := f.strs["from-file"]; hasFile {
		if text != "" {
			fatal("record: --from-file and a text argument are mutually exclusive")
		}
		cmdRecordFromFile(repo, path, f)
		return
	}
	if text == "" {
		fatal("record needs text")
	}
	b := openBrain(repo)
	ep := b.Record(text, buildOutcome(f))
	fmt.Printf("recorded %s\n", ep.ID)
}

// buildOutcome constructs the Outcome for a record from the --reward/--label/
// --dimension flags, or nil if --reward was not given. Returns a fresh value
// each call so callers recording multiple episodes (record --from-file) don't
// share one Outcome pointer across episodes.
func buildOutcome(f flags) *engine.Outcome {
	r, ok := f.floats["reward"]
	if !ok {
		return nil
	}
	return &engine.Outcome{Reward: r, Label: f.strs["label"], Dimension: f.strs["dimension"]}
}

// cmdRecordFromFile chunks a local file (ingest.ChunkFile) and records each
// chunk as its own episode, in file order. Deterministic and network-free:
// chunking is a pure function of the file's bytes.
func cmdRecordFromFile(repo, path string, f flags) {
	maxTokens := 450
	if v, ok := f.floats["max-tokens"]; ok {
		maxTokens = int(v)
	}
	overlap := 60
	if v, ok := f.floats["overlap"]; ok {
		overlap = int(v)
	}
	chunks, err := ingest.ChunkFile(path, maxTokens, overlap)
	if err != nil {
		fatal("record --from-file: %v", err)
	}
	b := openBrain(repo)
	ids := make([]string, 0, len(chunks))
	for _, c := range chunks {
		ep := b.Record(c.Text, buildOutcome(f))
		ids = append(ids, ep.ID)
		if !f.bools["json"] {
			fmt.Printf("recorded %s\n", ep.ID)
		}
	}
	if f.bools["json"] {
		emit(ids)
		return
	}
	fmt.Printf("recorded %d episodes from %s\n", len(ids), path)
}

func cmdReappraise(repo string, args []string) {
	if len(args) == 0 {
		fatal("reappraise needs an episode id")
	}
	id := args[0]
	f := parseFlags(args[1:])
	r, ok := f.floats["reward"]
	if !ok {
		fatal("reappraise needs --reward")
	}
	b := openBrain(repo)
	if !b.Reappraise(id, engine.Outcome{Reward: r, Label: f.strs["label"], Note: f.strs["note"]}) {
		fatal("unknown episode: %s", id)
	}
	fmt.Printf("reappraised %s\n", id)
}

func cmdRecall(repo string, args []string) {
	q, f := firstArgAndFlags(args)
	if q == "" {
		fatal("recall needs a query")
	}
	k := 5
	if v, ok := f.floats["k"]; ok {
		k = int(v)
	}
	r := openBrain(repo).Ask(q, k)
	if f.bools["json"] {
		emit(map[string]any{
			"grounded": r.Grounded, "conflict": r.Conflict, "answer": r.Answer,
			"reason": r.Reason, "supporting": len(r.Episodes),
		})
		return
	}
	fmt.Printf("grounded: %v  conflict: %v\n%s\n", r.Grounded, r.Conflict, r.Answer)
}

func cmdLearn(repo string, args []string) {
	topic, f := firstArgAndFlags(args)
	if topic == "" {
		fatal("learn needs a topic")
	}
	b := openBrain(repo)
	var matched []engine.Conviction
	for _, cv := range b.Convictions(false) {
		if sharesWord(topic, cv.Statement) {
			matched = append(matched, cv)
		}
	}
	if f.bools["json"] {
		emit(matched)
		return
	}
	if len(matched) == 0 {
		fmt.Println("nothing learned about that yet")
		return
	}
	for _, cv := range matched {
		fmt.Printf("- %s (confidence %.2f, n=%d)\n", cv.Statement, cv.Confidence, cv.SupportCount)
	}
}

func cmdCheck(repo string, args []string) {
	decision, f := firstArgAndFlags(args)
	if decision == "" {
		fatal("check needs a decision")
	}
	reward, ok := f.floats["reward"]
	if !ok {
		fatal("check needs --reward")
	}
	ctx := engine.DecisionContext{Text: decision, Signals: f.signals}
	v := openBrain(repo).Check(ctx, reward, f.strs["fallback"])
	if f.bools["json"] {
		emit(map[string]any{
			"allowed": v.Allowed, "alarm": v.Alarm, "vetoed_by": v.VetoedBy,
			"penalized_by": v.PenalizedBy, "adjusted_reward": v.AdjustedReward,
			"guaranteed": v.Guaranteed(), "fallback": v.Fallback, "reasons": v.Reasons,
		})
		return
	}
	fmt.Printf("allowed: %v  ALARM: %v\n", v.Allowed, v.Alarm)
	if len(v.VetoedBy) > 0 {
		fmt.Printf("vetoed by: %v\n", v.VetoedBy)
	}
	if v.Fallback != "" {
		fmt.Printf("instead: %s\n", v.Fallback)
	}
}

func cmdConsolidate(repo string, args []string) {
	f := parseFlags(args)
	minSupport := 2
	if v, ok := f.floats["min-support"]; ok {
		minSupport = int(v)
	}
	minConsistency := 0.66
	if v, ok := f.floats["min-consistency"]; ok {
		minConsistency = v
	}
	rep := openBrain(repo).Consolidate(minSupport, minConsistency, 1, f.bools["forget"], 8760)
	if f.bools["json"] {
		emit(rep)
		return
	}
	fmt.Printf("convictions formed: %d  conflicts: %d  decayed: %d  total: %d\n",
		rep.ConvictionsFormed, rep.ConflictsSurfaced, rep.EpisodesDecayed, rep.TotalConvictions)
}

func cmdConvictions(repo string, args []string) {
	f := parseFlags(args)
	cvs := openBrain(repo).Convictions(false)
	if f.bools["json"] {
		emit(cvs)
		return
	}
	if len(cvs) == 0 {
		fmt.Println("(no convictions yet — record repeated experience and consolidate)")
		return
	}
	for _, cv := range cvs {
		fmt.Printf("- [%s] %s (confidence %.2f, n=%d)\n", cv.Valence, cv.Statement, cv.Confidence, cv.SupportCount)
	}
}

// cmdInstallSkills writes the bundled agent skill into the Claude skills dir and
// seeds ~/.config/brain/config.json — everything the agent needs, from the binary
// itself (the skill + example config are embedded). Works on every OS.
func cmdInstallSkills(args []string) {
	f := parseFlags(args)
	home, err := os.UserHomeDir()
	if err != nil {
		fatal("install-skills: %v", err)
	}
	skillDir := f.strs["skill-dir"]
	if skillDir == "" {
		skillDir = filepath.Join(home, ".claude", "skills", "brain")
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		fatal("install-skills: %v", err)
	}
	skillPath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillPath, embeddedSkill, 0o644); err != nil {
		fatal("install-skills: %v", err)
	}
	fmt.Printf("installed skill: %s\n", skillPath)

	cfgDir := filepath.Join(home, ".config", "brain")
	cfgPath := filepath.Join(cfgDir, "config.json")
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		if err := os.MkdirAll(cfgDir, 0o755); err != nil {
			fatal("install-skills: %v", err)
		}
		if err := os.WriteFile(cfgPath, embeddedConfig, 0o644); err != nil {
			fatal("install-skills: %v", err)
		}
		fmt.Printf("seeded config: %s (edit it to point at your brain repos)\n", cfgPath)
	} else {
		fmt.Printf("kept existing config: %s\n", cfgPath)
	}
}

func cmdStatus(repo string) {
	b := openBrain(repo)
	obj := "(none)"
	if o, ok := b.Objective(); ok {
		obj = o.Text
	}
	fmt.Printf("repo        : %s\n", repo)
	fmt.Printf("objective   : %s\n", obj)
	fmt.Printf("convictions : %d\n", len(b.Convictions(false)))
}

// --- flag parsing (tiny, dependency-free) -----------------------------------

type flags struct {
	strs    map[string]string
	floats  map[string]float64
	bools   map[string]bool
	signals map[string]float64
}

func parseFlags(args []string) flags {
	f := flags{strs: map[string]string{}, floats: map[string]float64{}, bools: map[string]bool{}, signals: map[string]float64{}}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			continue
		}
		name := strings.TrimLeft(a, "-")
		if name == "json" || name == "forget" {
			f.bools[name] = true
			continue
		}
		if i+1 >= len(args) {
			continue
		}
		val := args[i+1]
		i++
		if name == "signal" {
			if k, v, ok := splitKV(val); ok {
				f.signals[k] = v
			}
			continue
		}
		if fv, err := strconv.ParseFloat(val, 64); err == nil {
			f.floats[name] = fv
		}
		f.strs[name] = val
	}
	return f
}

func firstArgAndFlags(args []string) (string, flags) {
	if len(args) == 0 || strings.HasPrefix(args[0], "-") {
		return "", parseFlags(args)
	}
	return args[0], parseFlags(args[1:])
}

func splitKV(s string) (string, float64, bool) {
	parts := strings.SplitN(s, "=", 2)
	if len(parts) != 2 {
		return "", 0, false
	}
	v, err := strconv.ParseFloat(parts[1], 64)
	return parts[0], v, err == nil
}

func sharesWord(a, b string) bool {
	set := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(a)) {
		set[w] = true
	}
	for _, w := range strings.Fields(strings.ToLower(b)) {
		if set[w] {
			return true
		}
	}
	return false
}

func emit(v any) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "brain: "+format+"\n", a...)
	os.Exit(1)
}

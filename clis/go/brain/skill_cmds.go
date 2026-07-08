// The skill-library verbs (Phase B): a dumb front over engine.SkillLib. The
// CLI is bookkeeping + the arithmetic gate only — the AGENT synthesizes skill
// content, runs held-out cases, and writes rationales (see the brain skill's
// SKILL.md). Library lives in <repo>/skills-lib/.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/muthuishere/brain/libs/go/engine"
)

func openSkillLib(repo string) *engine.SkillLib {
	l, err := engine.LoadSkillLib(filepath.Join(repo, "skills-lib"))
	if err != nil {
		fatal("skills-lib: %v", err)
	}
	return l
}

func saveSkillLib(l *engine.SkillLib) {
	if err := l.Save(); err != nil {
		fatal("skills-lib: %v", err)
	}
}

func cmdSkill(repo string, args []string) {
	if len(args) == 0 {
		fatal("skill needs a subcommand: register | validate | log | search | metrics | deprecate | rollback")
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "register":
		cmdSkillRegister(repo, rest)
	case "validate":
		cmdSkillValidate(repo, rest)
	case "log":
		cmdSkillLog(repo, rest)
	case "search":
		cmdSkillSearch(repo, rest)
	case "metrics":
		cmdSkillMetrics(repo, rest)
	case "deprecate":
		cmdSkillDeprecate(repo, rest)
	case "rollback":
		cmdSkillRollback(repo, rest)
	default:
		fatal("unknown skill subcommand %q", sub)
	}
}

func cmdSkillRegister(repo string, args []string) {
	f := parseFlags(args)
	id, from := f.strs["id"], f.strs["from"]
	if id == "" || from == "" {
		fatal("skill register needs --id and --from FILE")
	}
	content, err := os.ReadFile(from)
	if err != nil {
		fatal("skill register: %v", err)
	}
	l := openSkillLib(repo)
	s, version, err := l.RegisterSkill(id, f.strs["kind"], f.strs["description"], f.strs["domain"],
		f.strs["parent"], f.strs["rationale"], string(content), now())
	if err != nil {
		fatal("skill register: %v", err)
	}
	saveSkillLib(l)
	if f.bools["json"] {
		emit(map[string]any{"skill": s, "registered_version": version})
		return
	}
	if version == 1 {
		fmt.Printf("registered %s v1 (current)\n", id)
	} else {
		fmt.Printf("registered %s v%d as CANDIDATE (current stays v%d until `brain skill validate`)\n",
			id, version, s.CurrentVersion)
	}
}

func readResults(dir, name string) engine.TestResults {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		fatal("skill validate: %v", err)
	}
	var r engine.TestResults
	if err := json.Unmarshal(data, &r); err != nil {
		fatal("skill validate %s: %v", name, err)
	}
	return r
}

// cmdSkillValidate runs the promotion gate. --test-data DIR holds prior.json
// and candidate.json ({"passed":N,"failed":M}) — counts the AGENT measured by
// running held-out cases against each version. The threshold comes from
// evolve-policy.json (owner-owned), never from the run itself.
func cmdSkillValidate(repo string, args []string) {
	id, f := firstArgAndFlags(args)
	if id == "" {
		fatal("skill validate needs a skill id")
	}
	dir := f.strs["test-data"]
	if dir == "" {
		fatal("skill validate needs --test-data DIR (prior.json + candidate.json)")
	}
	prior, candidate := readResults(dir, "prior.json"), readResults(dir, "candidate.json")
	l := openSkillLib(repo)
	s := l.Find(id)
	if s == nil {
		fatal("unknown skill: %s", id)
	}
	candidateVersion := s.Versions[len(s.Versions)-1].Version
	policy := loadEvolvePolicy(repo)
	res, err := l.ValidateVersion(id, candidateVersion, prior, candidate, policy.MinImprovement, now())
	if err != nil {
		fatal("skill validate: %v", err)
	}
	saveSkillLib(l)
	if f.bools["json"] {
		emit(map[string]any{"skill_id": id, "candidate_version": candidateVersion, "result": res})
		return
	}
	verdict := "REJECTED (archived)"
	if res.Accepted {
		verdict = fmt.Sprintf("ACCEPTED — v%d is now current", candidateVersion)
	}
	fmt.Printf("%s: %s (prior %.2f → candidate %.2f, %s)\n", id, verdict, res.PriorRate, res.CandidateRate, res.Reason)
}

func cmdSkillLog(repo string, args []string) {
	id, f := firstArgAndFlags(args)
	if id == "" {
		fatal("skill log needs a skill id")
	}
	outcome := f.strs["outcome"]
	if outcome == "" {
		fatal("skill log needs --outcome ok|fail")
	}
	l := openSkillLib(repo)
	err := l.LogUsage(engine.UsageEvent{
		SkillID: id, Task: f.strs["task"], Outcome: outcome, Cost: f.floats["cost"], Ts: now(),
	})
	if err != nil {
		fatal("skill log: %v", err)
	}
	saveSkillLib(l)
	s := l.Find(id)
	if f.bools["json"] {
		emit(s.Metrics)
		return
	}
	fmt.Printf("logged %s %s — success %.2f over %d uses\n", id, outcome, s.Metrics.SuccessRate, s.Metrics.InvocationCount)
}

func cmdSkillSearch(repo string, args []string) {
	f := parseFlags(args)
	l := openSkillLib(repo)
	skills := l.Search(f.strs["domain"], f.floats["min-success"])
	if f.bools["json"] {
		emit(skills)
		return
	}
	if len(skills) == 0 {
		fmt.Println("(no matching active skills)")
		return
	}
	for _, s := range skills {
		fmt.Printf("- %s v%d [%s/%s] success %.2f over %d uses — %s\n",
			s.ID, s.CurrentVersion, s.Kind, s.Domain, s.Metrics.SuccessRate, s.Metrics.InvocationCount, s.Description)
	}
}

func cmdSkillMetrics(repo string, args []string) {
	id, f := firstArgAndFlags(args)
	if id == "" {
		fatal("skill metrics needs a skill id")
	}
	m, err := openSkillLib(repo).MetricsWindow(id, f.floats["since"])
	if err != nil {
		fatal("skill metrics: %v", err)
	}
	if f.bools["json"] {
		emit(m)
		return
	}
	fmt.Printf("%s: success %.2f  uses %d  cost %.2f\n", id, m.SuccessRate, m.InvocationCount, m.TotalCost)
}

func cmdSkillDeprecate(repo string, args []string) {
	id, f := firstArgAndFlags(args)
	if id == "" {
		fatal("skill deprecate needs a skill id")
	}
	l := openSkillLib(repo)
	if err := l.Deprecate(id); err != nil {
		fatal("skill deprecate: %v", err)
	}
	saveSkillLib(l)
	if f.bools["json"] {
		emit(l.Find(id))
		return
	}
	fmt.Printf("deprecated %s (files and lineage kept — rollback possible)\n", id)
}

func cmdSkillRollback(repo string, args []string) {
	id, f := firstArgAndFlags(args)
	if id == "" {
		fatal("skill rollback needs a skill id")
	}
	to, ok := f.floats["to"]
	if !ok {
		fatal("skill rollback needs --to VERSION")
	}
	l := openSkillLib(repo)
	if err := l.Rollback(id, int(to)); err != nil {
		fatal("skill rollback: %v", err)
	}
	saveSkillLib(l)
	if f.bools["json"] {
		emit(l.Find(id))
		return
	}
	fmt.Printf("rolled %s back to v%d\n", id, int(to))
}

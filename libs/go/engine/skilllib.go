package engine

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/muthuishere/citenexus/golang/gate"
)

// Phase B of the self-evolving brain: a versioned SKILL LIBRARY (SkillOpt,
// arXiv 2605.23904) inside the brain repo. A skill is an executable procedure
// spec; every change carries a rationale (explainability), every version keeps
// lineage, and promotion goes through an arithmetic validation gate. The AGENT
// runs the held-out cases (it is the LLM); this core is only the bookkeeping
// and the gate — deterministic, model-free, git-friendly plain text:
//
//	skills-lib/primitives/<id>/v<N>.md    the spec content per version
//	skills-lib/composites/<id>/v<N>.md    composites chain primitives
//	skills-lib/archived/<id>/v<N>.md      rejected/retired versions (kept, never deleted)
//	skills-lib/metadata.json              the index: versions, lineage, metrics
//	skills-lib/usage.ndjson               append-only per-use feedback log

// SkillMetrics is the health of a skill, recomputed from the usage log.
type SkillMetrics struct {
	SuccessRate     float64 `json:"success_rate"`
	InvocationCount int     `json:"invocation_count"`
	TotalCost       float64 `json:"total_cost"`
}

// ValidationResult is the gate's ruling on a candidate version.
type ValidationResult struct {
	Accepted      bool    `json:"accepted"`
	PriorRate     float64 `json:"prior_rate"`
	CandidateRate float64 `json:"candidate_rate"`
	Improvement   float64 `json:"improvement"`
	Reason        string  `json:"reason"`
	Ts            float64 `json:"ts"`
}

// SkillVersion is one iteration of a skill's spec, with its why and its verdict.
type SkillVersion struct {
	Version    int               `json:"version"`
	File       string            `json:"file"` // relative to skills-lib/
	Rationale  string            `json:"rationale"`
	CreatedTs  float64           `json:"created_ts"`
	Validation *ValidationResult `json:"validation,omitempty"`
}

// Skill is one entry in the library: an executable procedure spec with versions,
// lineage, and live metrics.
type Skill struct {
	ID             string         `json:"id"`
	Kind           string         `json:"kind"` // primitive | composite
	Description    string         `json:"description,omitempty"`
	Domain         string         `json:"domain,omitempty"`
	Status         string         `json:"status"` // active | deprecated
	CurrentVersion int            `json:"current_version"`
	Versions       []SkillVersion `json:"versions"`
	Metrics        SkillMetrics   `json:"metrics"`
	Parent         string         `json:"parent,omitempty"` // lineage: synthesized from this skill
}

// Active reports whether the skill is in force.
func (s Skill) Active() bool { return s.Status == "active" }

// UsageEvent is one per-use feedback record (agents log after each invocation).
type UsageEvent struct {
	SkillID string  `json:"skill_id"`
	Version int     `json:"version"`
	Task    string  `json:"task"`
	Outcome string  `json:"outcome"` // ok | fail
	Cost    float64 `json:"cost"`
	Ts      float64 `json:"ts"`
}

// TestResults are held-out pass/fail counts the agent measured for one version.
type TestResults struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

// Rate is the success rate; an empty result set rates 0.
func (r TestResults) Rate() float64 {
	total := r.Passed + r.Failed
	if total == 0 {
		return 0
	}
	return float64(r.Passed) / float64(total)
}

// SkillLib is the library loaded from <brain-repo>/skills-lib.
type SkillLib struct {
	dir    string
	Skills []Skill `json:"skills"`
}

// LoadSkillLib opens (creating if needed) the skills-lib folder and its index.
func LoadSkillLib(dir string) (*SkillLib, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	l := &SkillLib{dir: dir}
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return l, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, l); err != nil {
		return nil, err
	}
	return l, nil
}

// Save persists the index. Sorted by id so the file is git-stable.
func (l *SkillLib) Save() error {
	sort.SliceStable(l.Skills, func(i, j int) bool { return l.Skills[i].ID < l.Skills[j].ID })
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(l.dir, "metadata.json"), data, 0o644)
}

// Find returns a pointer into the library for in-place mutation.
func (l *SkillLib) Find(id string) *Skill {
	for i := range l.Skills {
		if l.Skills[i].ID == id {
			return &l.Skills[i]
		}
	}
	return nil
}

func kindDir(kind string) string {
	if kind == "composite" {
		return "composites"
	}
	return "primitives"
}

// RegisterSkill adds a new skill (v1 becomes current) or a new candidate
// version of an existing one (current is NOT promoted — validate does that).
// Rationale is mandatory: every change stores its why.
func (l *SkillLib) RegisterSkill(id, kind, description, domain, parent, rationale, content string, now float64) (*Skill, int, error) {
	if strings.TrimSpace(rationale) == "" {
		return nil, 0, errors.New("rationale is required — every skill change stores its why")
	}
	// The id becomes a path segment — a traversal id could write outside the
	// library (even the owner-owned policy file). Reject anything path-like.
	if id == "" || id != filepath.Base(id) || id == "." || id == ".." ||
		strings.ContainsAny(id, `/\`) {
		return nil, 0, fmt.Errorf("invalid skill id %q: must be a plain name, no path separators", id)
	}
	if kind != "primitive" && kind != "composite" {
		kind = "primitive"
	}
	s := l.Find(id)
	version := 1
	if s != nil {
		if s.Kind != kind && kind != "primitive" { // kind is fixed at birth
			return nil, 0, fmt.Errorf("skill %s is a %s; kind cannot change", id, s.Kind)
		}
		version = s.Versions[len(s.Versions)-1].Version + 1
	}
	rel := filepath.Join(kindDir(kindOf(s, kind)), id, fmt.Sprintf("v%d.md", version))
	abs := filepath.Join(l.dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return nil, 0, err
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		return nil, 0, err
	}
	sv := SkillVersion{Version: version, File: rel, Rationale: rationale, CreatedTs: now}
	if s == nil {
		l.Skills = append(l.Skills, Skill{
			ID: id, Kind: kind, Description: description, Domain: domain,
			Status: "active", CurrentVersion: 1, Versions: []SkillVersion{sv}, Parent: parent,
		})
		return l.Find(id), version, nil
	}
	s.Versions = append(s.Versions, sv)
	if description != "" {
		s.Description = description
	}
	if domain != "" {
		s.Domain = domain
	}
	return s, version, nil
}

func kindOf(s *Skill, fallback string) string {
	if s != nil {
		return s.Kind
	}
	return fallback
}

// ValidateVersion is the gate (evaluator OUTSIDE the loop): accept the candidate
// only if its held-out success rate beats the prior version's by at least
// minImprovement. Accepted → promoted to current. Rejected → the version stays
// in metadata with its verdict and its file moves to archived/ — kept for
// learning, never silently current.
func (l *SkillLib) ValidateVersion(id string, candidateVersion int, prior, candidate TestResults, minImprovement, now float64) (ValidationResult, error) {
	s := l.Find(id)
	if s == nil {
		return ValidationResult{}, fmt.Errorf("unknown skill: %s", id)
	}
	var sv *SkillVersion
	for i := range s.Versions {
		if s.Versions[i].Version == candidateVersion {
			sv = &s.Versions[i]
		}
	}
	if sv == nil {
		return ValidationResult{}, fmt.Errorf("skill %s has no version %d", id, candidateVersion)
	}
	// The current version is not a candidate: rejecting it would archive the
	// live spec while the skill stays active on it. Register a new version first.
	if candidateVersion == s.CurrentVersion {
		return ValidationResult{}, fmt.Errorf("version %d is already current for %s — register a new candidate version first", candidateVersion, id)
	}
	res := ValidationResult{
		PriorRate:     prior.Rate(),
		CandidateRate: candidate.Rate(),
		Improvement:   candidate.Rate() - prior.Rate(),
		Ts:            now,
	}
	if res.Improvement >= minImprovement {
		res.Accepted = true
		res.Reason = fmt.Sprintf("improvement %.3f >= threshold %.3f", res.Improvement, minImprovement)
		s.CurrentVersion = candidateVersion
	} else {
		res.Reason = fmt.Sprintf("improvement %.3f below threshold %.3f — archived", res.Improvement, minImprovement)
		if err := l.archiveVersionFile(s.ID, sv); err != nil {
			return res, err
		}
	}
	sv.Validation = &res
	return res, nil
}

func (l *SkillLib) archiveVersionFile(id string, sv *SkillVersion) error {
	rel := filepath.Join("archived", id, filepath.Base(sv.File))
	abs := filepath.Join(l.dir, rel)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	if err := os.Rename(filepath.Join(l.dir, sv.File), abs); err != nil {
		return err
	}
	sv.File = rel
	return nil
}

// LogUsage appends one per-use feedback event and recomputes the skill's
// metrics from the full usage log (the log is the source of truth).
func (l *SkillLib) LogUsage(ev UsageEvent) error {
	s := l.Find(ev.SkillID)
	if s == nil {
		return fmt.Errorf("unknown skill: %s", ev.SkillID)
	}
	if ev.Outcome != "ok" && ev.Outcome != "fail" {
		return fmt.Errorf("outcome must be ok or fail, got %q", ev.Outcome)
	}
	if ev.Version == 0 {
		ev.Version = s.CurrentVersion
	}
	f, err := os.OpenFile(filepath.Join(l.dir, "usage.ndjson"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	line, _ := json.Marshal(ev)
	_, err = f.Write(append(line, '\n'))
	f.Close()
	if err != nil {
		return err
	}
	events, err := l.readUsage()
	if err != nil {
		return err
	}
	s.Metrics = metricsOf(events, ev.SkillID, 0)
	return nil
}

// MetricsWindow computes a skill's metrics from usage since sinceTs (0 = all).
func (l *SkillLib) MetricsWindow(id string, sinceTs float64) (SkillMetrics, error) {
	if l.Find(id) == nil {
		return SkillMetrics{}, fmt.Errorf("unknown skill: %s", id)
	}
	events, err := l.readUsage()
	if err != nil {
		return SkillMetrics{}, err
	}
	return metricsOf(events, id, sinceTs), nil
}

func metricsOf(events []UsageEvent, id string, sinceTs float64) SkillMetrics {
	var m SkillMetrics
	ok := 0
	for _, ev := range events {
		if ev.SkillID != id || ev.Ts < sinceTs {
			continue
		}
		m.InvocationCount++
		m.TotalCost += ev.Cost
		if ev.Outcome == "ok" {
			ok++
		}
	}
	if m.InvocationCount > 0 {
		m.SuccessRate = float64(ok) / float64(m.InvocationCount)
	}
	return m
}

func (l *SkillLib) readUsage() ([]UsageEvent, error) {
	f, err := os.Open(filepath.Join(l.dir, "usage.ndjson"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()
	var out []UsageEvent
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1024*1024), 8*1024*1024)
	for sc.Scan() {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var ev UsageEvent
		if err := json.Unmarshal(sc.Bytes(), &ev); err != nil {
			return nil, err
		}
		out = append(out, ev)
	}
	return out, sc.Err()
}

// Search returns active skills matching a domain (substring or shared content
// token; empty matches all) with at least minSuccess success rate.
func (l *SkillLib) Search(domain string, minSuccess float64) []Skill {
	dToks := gate.ContentTokens(domain)
	var out []Skill
	for _, s := range l.Skills {
		if !s.Active() || s.Metrics.SuccessRate < minSuccess {
			continue
		}
		if domain != "" {
			hay := strings.ToLower(s.Domain + " " + s.Description + " " + s.ID)
			if !strings.Contains(hay, strings.ToLower(domain)) &&
				sharedCount(dToks, gate.ContentTokens(hay)) < 1 {
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

// Deprecate retires a low-usage/degraded skill. Files and lineage are kept —
// rollback stays possible; nothing is deleted.
func (l *SkillLib) Deprecate(id string) error {
	s := l.Find(id)
	if s == nil {
		return fmt.Errorf("unknown skill: %s", id)
	}
	s.Status = "deprecated"
	return nil
}

// Rollback points a skill back at an earlier accepted version.
func (l *SkillLib) Rollback(id string, to int) error {
	s := l.Find(id)
	if s == nil {
		return fmt.Errorf("unknown skill: %s", id)
	}
	for _, sv := range s.Versions {
		if sv.Version != to {
			continue
		}
		if sv.Validation != nil && !sv.Validation.Accepted {
			return fmt.Errorf("version %d was rejected by the gate — cannot roll back to it", to)
		}
		s.CurrentVersion = to
		s.Status = "active"
		return nil
	}
	return fmt.Errorf("skill %s has no version %d", id, to)
}

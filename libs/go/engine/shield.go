package engine

import "sort"

// The constraint shield — a deterministic veto that sits outside the reasoning,
// mirroring src/citenexus/brain/shield.py. No model is ever consulted here: the
// creative component must not be its own safety arbiter (Alshiekh et al. 2018).
// The signature event is "objective reward is high AND a constraint is violated"
// — profitable-but-ruinous — flagged as the loudest alarm the brain can raise.

// ConstraintKind is how hard a constraint bites.
type ConstraintKind int

const (
	// Hard constraints veto a violating decision outright.
	Hard ConstraintKind = iota
	// Soft constraints penalize but do not block.
	Soft
)

// Provenance is where a constraint's cost actually came from — the shield's
// SPEC-shield-signal-provenance-v1 fix. Absence of evidence is never evidence
// of safety: a signal a constraint declares but that the caller never supplied
// must never be indistinguishable from a signal that was supplied and is 0.
type Provenance int

const (
	// Computed: cost came from a Go CostFn (Check). Trustworthy.
	Computed Provenance = iota
	// Provided: cost came from ctx.Signals[Signal], and the key was present.
	// Trustworthy — the caller actually measured and supplied it.
	Provided
	// Absent: the constraint names a Signal, but the caller did not supply it.
	// The cost is unknown, not zero.
	Absent
	// Unbound: the constraint has neither a Check nor a Signal — it declares
	// no evaluable cost source at all. The cost is unknown, not zero.
	Unbound
)

// Determined reports whether this provenance is a trustworthy, measured cost
// (Computed or Provided) as opposed to an unknown one (Absent or Unbound).
func (p Provenance) Determined() bool { return p == Computed || p == Provided }

// WhenAbsent policies govern what an undetermined (Absent/Unbound) cost means
// for a constraint. Empty resolves to the kind default: "veto" for Hard,
// "assume_safe" for Soft.
const (
	WhenAbsentVeto       = "veto"
	WhenAbsentAbstain    = "abstain"
	WhenAbsentAssumeSafe = "assume_safe"
)

// DecisionContext is a proposed decision plus measured signals a check can read.
type DecisionContext struct {
	Text    string
	Signals map[string]float64
}

// CostFn is a constraint's deterministic cost function (0 = satisfied, higher = worse).
type CostFn func(DecisionContext) float64

// Constraint is a standing background rule with veto (hard) or penalty (soft) power.
//
// A cost may come from a Go CostFn (Check) or, for the serializable/declarative
// case (constraints.json in a repo), from a named Signal read off the decision's
// Signals map. Either way the cost is deterministic — the shield never asks a model.
type Constraint struct {
	Name       string
	Text       string
	Kind       ConstraintKind
	Threshold  float64
	Weight     float64
	Signal     string `json:",omitempty"` // read cost from ctx.Signals[Signal]
	Check      CostFn `json:"-"`          // or compute it in code (wins over Signal)
	WhenAbsent string `json:",omitempty"` // "veto" | "abstain" | "assume_safe"; empty = kind default
}

// effectiveWhenAbsent resolves the configured policy against this constraint's kind.
func (c Constraint) effectiveWhenAbsent() string {
	if c.WhenAbsent != "" {
		return c.WhenAbsent
	}
	if c.Kind == Hard {
		return WhenAbsentVeto
	}
	return WhenAbsentAssumeSafe
}

// cost returns this constraint's cost for a decision and its provenance — where
// that cost actually came from. A missing Signal key is Absent (unknown), never
// silently 0-and-trusted; a Constraint with no Check and no Signal is Unbound.
func (c Constraint) cost(ctx DecisionContext) (float64, Provenance) {
	if c.Check != nil {
		return c.Check(ctx), Computed
	}
	if c.Signal != "" {
		if v, ok := ctx.Signals[c.Signal]; ok {
			return v, Provided
		}
		return 0, Absent
	}
	return 0, Unbound
}

// ConstraintEval is the result of weighing one constraint against a decision.
type ConstraintEval struct {
	Name         string
	Kind         ConstraintKind
	Cost         float64
	Threshold    float64
	Violated     bool
	Provenance   Provenance
	Undetermined bool // true when Provenance is Absent/Unbound and it mattered (see Evaluate)
}

// Deterministic reports whether this evaluation's cost was actually measured
// (Computed or Provided), matching the pre-fix field name for readability.
func (e ConstraintEval) Deterministic() bool { return e.Provenance.Determined() }

// Verdict is the shield's ruling on a proposed decision.
type Verdict struct {
	Decision        string
	Allowed         bool
	Alarm           bool
	Undetermined    bool     // ≥1 hard constraint's required signal was never supplied
	UndeterminedBy  []string // names of those constraints, sorted
	ObjectiveReward float64
	AdjustedReward  float64
	VetoedBy        []string
	PenalizedBy     []string
	Evaluations     []ConstraintEval
	Reasons         []string
	Fallback        string
}

// Guaranteed reports whether every constraint that mattered had its cost
// actually determined from a real input — not merely that the code path ran.
// A verdict can be Allowed && !Guaranteed: everything provided cleared, but
// something was assume_safe/Unbound and so advisory, not proven.
func (v Verdict) Guaranteed() bool {
	for _, e := range v.Evaluations {
		if !e.Provenance.Determined() {
			return false
		}
	}
	return true
}

// Shield is a deterministic post-shield over a set of standing constraints.
type Shield struct {
	constraints []Constraint
	highReward  float64
}

// Evaluate rules on a decision: allow, penalize, or veto — by hard comparison.
//
// Absence of evidence is never evidence of safety (SPEC-shield-signal-provenance-v1):
// a constraint whose cost could not be determined (its named Signal was never
// supplied, or it declares no cost source at all) is never treated as satisfied.
// A Hard constraint's default policy fails closed — Undetermined, not Allowed —
// exactly when omitting the signal would otherwise have let the decision through.
func (s Shield) Evaluate(ctx DecisionContext, objectiveReward float64, fallback string) Verdict {
	var (
		evals        []ConstraintEval
		vetoed       []string
		penalized    []string
		undetermined []string
		reasons      []string
		penalty      float64
	)
	for _, c := range s.constraints {
		cost, prov := c.cost(ctx)
		determined := prov.Determined()
		violated := determined && cost > c.Threshold
		eval := ConstraintEval{
			Name: c.Name, Kind: c.Kind, Cost: cost, Threshold: c.Threshold,
			Violated: violated, Provenance: prov,
		}

		switch {
		case determined && violated:
			if c.Kind == Hard {
				vetoed = append(vetoed, c.Name)
				reasons = append(reasons, "hard constraint violated: "+c.Text)
			} else {
				penalized = append(penalized, c.Name)
				penalty += c.Weight * (cost - c.Threshold)
				reasons = append(reasons, "soft constraint strained: "+c.Text)
			}
		case determined:
			// Satisfied — cost was measured and within threshold. Nothing to do.
		case prov == Unbound:
			// Unbound always behaves as assume_safe: it cannot fail closed on a
			// cost source that was never declared. Guaranteed() still catches it.
		case c.effectiveWhenAbsent() == WhenAbsentAssumeSafe:
			// Absent, but the constraint explicitly opts into treating an
			// unsupplied signal as safe. Guaranteed() still catches it.
		case c.Kind == Hard:
			// Absent required signal on a Hard constraint, policy veto/abstain:
			// fail closed. Not added to VetoedBy — nothing was actually measured
			// to have violated — but the decision is not allowed.
			eval.Undetermined = true
			undetermined = append(undetermined, c.Name)
			reasons = append(reasons, "undetermined: required signal '"+c.Signal+
				"' for hard constraint '"+c.Name+"' not provided")
		default:
			// Absent on a Soft constraint with an explicit non-assume_safe
			// policy: skip the penalty, but it's still undetermined (non-det).
			eval.Undetermined = true
			undetermined = append(undetermined, c.Name)
		}
		evals = append(evals, eval)
	}
	sort.Strings(vetoed)
	sort.Strings(penalized)
	sort.Strings(undetermined)

	isUndetermined := len(undetermined) > 0
	allowed := len(vetoed) == 0 && !isUndetermined
	anyViolation := len(vetoed) > 0 || len(penalized) > 0
	alarm := objectiveReward >= s.highReward && (anyViolation || isUndetermined)
	if alarm {
		reasons = append([]string{
			"profitable-but-ruinous: high objective reward with a violated constraint",
		}, reasons...)
	}
	fb := ""
	if len(vetoed) > 0 || isUndetermined {
		fb = fallback
	}
	return Verdict{
		Decision: ctx.Text, Allowed: allowed, Alarm: alarm,
		Undetermined: isUndetermined, UndeterminedBy: undetermined,
		ObjectiveReward: objectiveReward, AdjustedReward: objectiveReward - penalty,
		VetoedBy: vetoed, PenalizedBy: penalized, Evaluations: evals,
		Reasons: reasons, Fallback: fb,
	}
}

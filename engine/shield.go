package engine

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
	Name      string
	Text      string
	Kind      ConstraintKind
	Threshold float64
	Weight    float64
	Signal    string `json:",omitempty"` // read cost from ctx.Signals[Signal]
	Check     CostFn `json:"-"`          // or compute it in code (wins over Signal)
}

// cost returns the deterministic cost of this constraint for a decision, and
// whether it was computed (Check or Signal) rather than left to an external value.
func (c Constraint) cost(ctx DecisionContext) (float64, bool) {
	if c.Check != nil {
		return c.Check(ctx), true
	}
	if c.Signal != "" {
		return ctx.Signals[c.Signal], true
	}
	return 0, false
}

// ConstraintEval is the result of weighing one constraint against a decision.
type ConstraintEval struct {
	Name          string
	Kind          ConstraintKind
	Cost          float64
	Threshold     float64
	Violated      bool
	Deterministic bool
}

// Verdict is the shield's ruling on a proposed decision.
type Verdict struct {
	Decision        string
	Allowed         bool
	Alarm           bool
	ObjectiveReward float64
	AdjustedReward  float64
	VetoedBy        []string
	PenalizedBy     []string
	Evaluations     []ConstraintEval
	Reasons         []string
	Fallback        string
}

// Guaranteed reports whether every constraint that mattered self-evaluated in code.
func (v Verdict) Guaranteed() bool {
	for _, e := range v.Evaluations {
		if !e.Deterministic {
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
func (s Shield) Evaluate(ctx DecisionContext, objectiveReward float64, fallback string) Verdict {
	var (
		evals     []ConstraintEval
		vetoed    []string
		penalized []string
		reasons   []string
		penalty   float64
	)
	for _, c := range s.constraints {
		cost, deterministic := c.cost(ctx)
		violated := cost > c.Threshold
		evals = append(evals, ConstraintEval{
			Name: c.Name, Kind: c.Kind, Cost: cost, Threshold: c.Threshold,
			Violated: violated, Deterministic: deterministic,
		})
		if !violated {
			continue
		}
		if c.Kind == Hard {
			vetoed = append(vetoed, c.Name)
			reasons = append(reasons, "hard constraint violated: "+c.Text)
		} else {
			penalized = append(penalized, c.Name)
			penalty += c.Weight * (cost - c.Threshold)
			reasons = append(reasons, "soft constraint strained: "+c.Text)
		}
	}
	allowed := len(vetoed) == 0
	anyViolation := len(vetoed) > 0 || len(penalized) > 0
	alarm := objectiveReward >= s.highReward && anyViolation
	if alarm {
		reasons = append([]string{
			"profitable-but-ruinous: high objective reward with a violated constraint",
		}, reasons...)
	}
	fb := ""
	if len(vetoed) > 0 {
		fb = fallback
	}
	return Verdict{
		Decision: ctx.Text, Allowed: allowed, Alarm: alarm,
		ObjectiveReward: objectiveReward, AdjustedReward: objectiveReward - penalty,
		VetoedBy: vetoed, PenalizedBy: penalized, Evaluations: evals,
		Reasons: reasons, Fallback: fb,
	}
}

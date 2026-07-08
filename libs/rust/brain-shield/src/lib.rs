//! The constraint shield — a deterministic veto that sits outside the reasoning,
//! ported line-for-line from `libs/go/engine/shield.go`. No model is ever
//! consulted here: the creative component must not be its own safety arbiter
//! (Alshiekh et al. 2018). The signature event is "objective reward is high
//! AND a constraint is violated" — profitable-but-ruinous — flagged as the
//! loudest alarm the brain can raise.
//!
//! This crate is intentionally free of any serialization concerns: `evaluate()`
//! operates on plain structs/enums only. Fixture/JSON parsing for conformance
//! testing lives in `tests/conformance.rs` and depends on `serde`/`serde_json`
//! as dev-dependencies only.

use std::collections::HashMap;

/// How hard a constraint bites.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ConstraintKind {
    /// Hard constraints veto a violating decision outright.
    Hard,
    /// Soft constraints penalize but do not block.
    Soft,
}

/// Provenance is where a constraint's cost actually came from — the shield's
/// SPEC-shield-signal-provenance-v1 fix. Absence of evidence is never evidence
/// of safety: a signal a constraint declares but that the caller never supplied
/// must never be indistinguishable from a signal that was supplied and is 0.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Provenance {
    /// Cost came from a Rust cost function (`Check`). Trustworthy.
    Computed,
    /// Cost came from `ctx.signals[signal]`, and the key was present.
    /// Trustworthy — the caller actually measured and supplied it.
    Provided,
    /// The constraint names a Signal, but the caller did not supply it.
    /// The cost is unknown, not zero.
    Absent,
    /// The constraint has neither a Check nor a Signal — it declares no
    /// evaluable cost source at all. The cost is unknown, not zero.
    Unbound,
}

impl Provenance {
    /// Reports whether this provenance is a trustworthy, measured cost
    /// (Computed or Provided) as opposed to an unknown one (Absent or Unbound).
    pub fn determined(&self) -> bool {
        matches!(self, Provenance::Computed | Provenance::Provided)
    }
}

/// WhenAbsent policies govern what an undetermined (Absent/Unbound) cost means
/// for a constraint. `None` on `Constraint::when_absent` resolves to the kind
/// default: `Veto` for Hard, `AssumeSafe` for Soft.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WhenAbsent {
    Veto,
    Abstain,
    AssumeSafe,
}

impl WhenAbsent {
    /// The kind default: `Veto` for Hard, `AssumeSafe` for Soft.
    pub fn default_for(kind: ConstraintKind) -> WhenAbsent {
        match kind {
            ConstraintKind::Hard => WhenAbsent::Veto,
            ConstraintKind::Soft => WhenAbsent::AssumeSafe,
        }
    }
}

/// A constraint's deterministic cost function (0 = satisfied, higher = worse).
pub type CostFn = Box<dyn Fn(&DecisionContext) -> f64>;

/// DecisionContext is a proposed decision plus measured signals a check can read.
#[derive(Debug, Clone, Default)]
pub struct DecisionContext {
    pub text: String,
    pub signals: HashMap<String, f64>,
}

/// Constraint is a standing background rule with veto (hard) or penalty (soft) power.
///
/// A cost may come from a Rust `CostFn` (`check`) or, for the serializable/
/// declarative case (`constraints.json` in a repo), from a named Signal read
/// off the decision's `signals` map. Either way the cost is deterministic —
/// the shield never asks a model.
pub struct Constraint {
    pub name: String,
    pub text: String,
    pub kind: ConstraintKind,
    pub threshold: f64,
    pub weight: f64,
    /// Read cost from `ctx.signals[signal]`.
    pub signal: Option<String>,
    /// Or compute it in code (wins over `signal`).
    pub check: Option<CostFn>,
    /// `Veto` | `Abstain` | `AssumeSafe`; `None` = kind default.
    pub when_absent: Option<WhenAbsent>,
}

impl Constraint {
    /// A convenience constructor for the common declarative (signal-based, no
    /// `check`) case, since `CostFn` closures aren't needed by most callers.
    pub fn new(name: impl Into<String>, kind: ConstraintKind, threshold: f64) -> Self {
        Constraint {
            name: name.into(),
            text: String::new(),
            kind,
            threshold,
            weight: 1.0,
            signal: None,
            check: None,
            when_absent: None,
        }
    }

    /// Resolves the configured policy against this constraint's kind.
    fn effective_when_absent(&self) -> WhenAbsent {
        self.when_absent
            .unwrap_or_else(|| WhenAbsent::default_for(self.kind))
    }

    /// Returns this constraint's cost for a decision and its provenance —
    /// where that cost actually came from. A missing Signal key is Absent
    /// (unknown), never silently 0-and-trusted; a Constraint with no Check
    /// and no Signal is Unbound.
    fn cost(&self, ctx: &DecisionContext) -> (f64, Provenance) {
        if let Some(check) = &self.check {
            return (check(ctx), Provenance::Computed);
        }
        if let Some(signal) = &self.signal {
            if let Some(v) = ctx.signals.get(signal) {
                return (*v, Provenance::Provided);
            }
            return (0.0, Provenance::Absent);
        }
        (0.0, Provenance::Unbound)
    }
}

/// ConstraintEval is the result of weighing one constraint against a decision.
#[derive(Debug, Clone)]
pub struct ConstraintEval {
    pub name: String,
    pub kind: ConstraintKind,
    pub cost: f64,
    pub threshold: f64,
    pub violated: bool,
    pub provenance: Provenance,
    /// True when Provenance is Absent/Unbound and it mattered (see `evaluate`).
    pub undetermined: bool,
}

/// Verdict is the shield's ruling on a proposed decision.
#[derive(Debug, Clone)]
pub struct Verdict {
    pub decision: String,
    pub allowed: bool,
    pub alarm: bool,
    /// ≥1 hard constraint's required signal was never supplied.
    pub undetermined: bool,
    /// Names of those constraints, sorted.
    pub undetermined_by: Vec<String>,
    pub objective_reward: f64,
    pub adjusted_reward: f64,
    pub vetoed_by: Vec<String>,
    pub penalized_by: Vec<String>,
    pub evaluations: Vec<ConstraintEval>,
    pub reasons: Vec<String>,
    pub fallback: String,
}

impl Verdict {
    /// Reports whether every constraint that mattered had its cost actually
    /// determined from a real input — not merely that the code path ran.
    /// A verdict can be `allowed && !guaranteed`: everything provided cleared,
    /// but something was assume_safe/Unbound and so advisory, not proven.
    pub fn guaranteed(&self) -> bool {
        self.evaluations.iter().all(|e| e.provenance.determined())
    }
}

/// Shield is a deterministic post-shield over a set of standing constraints.
pub struct Shield {
    pub constraints: Vec<Constraint>,
    pub high_reward: f64,
}

impl Shield {
    /// Rules on a decision: allow, penalize, or veto — by hard comparison.
    ///
    /// Absence of evidence is never evidence of safety
    /// (SPEC-shield-signal-provenance-v1): a constraint whose cost could not
    /// be determined (its named Signal was never supplied, or it declares no
    /// cost source at all) is never treated as satisfied. A Hard constraint's
    /// default policy fails closed — Undetermined, not Allowed — exactly when
    /// omitting the signal would otherwise have let the decision through.
    pub fn evaluate(
        &self,
        ctx: &DecisionContext,
        objective_reward: f64,
        fallback: &str,
    ) -> Verdict {
        let mut evals: Vec<ConstraintEval> = Vec::new();
        let mut vetoed: Vec<String> = Vec::new();
        let mut penalized: Vec<String> = Vec::new();
        let mut undetermined: Vec<String> = Vec::new();
        let mut reasons: Vec<String> = Vec::new();
        let mut penalty: f64 = 0.0;

        for c in &self.constraints {
            let (cost, prov) = c.cost(ctx);
            let determined = prov.determined();
            let violated = determined && cost > c.threshold;
            let mut eval = ConstraintEval {
                name: c.name.clone(),
                kind: c.kind,
                cost,
                threshold: c.threshold,
                violated,
                provenance: prov,
                undetermined: false,
            };

            if determined && violated {
                if c.kind == ConstraintKind::Hard {
                    vetoed.push(c.name.clone());
                    reasons.push(format!("hard constraint violated: {}", c.text));
                } else {
                    penalized.push(c.name.clone());
                    penalty += c.weight * (cost - c.threshold);
                    reasons.push(format!("soft constraint strained: {}", c.text));
                }
            } else if determined {
                // Satisfied — cost was measured and within threshold. Nothing to do.
            } else if matches!(prov, Provenance::Unbound) {
                // Unbound always behaves as assume_safe: it cannot fail closed on a
                // cost source that was never declared. guaranteed() still catches it.
            } else if c.effective_when_absent() == WhenAbsent::AssumeSafe {
                // Absent, but the constraint explicitly opts into treating an
                // unsupplied signal as safe. guaranteed() still catches it.
            } else if c.kind == ConstraintKind::Hard {
                // Absent required signal on a Hard constraint, policy veto/abstain:
                // fail closed. Not added to vetoed_by — nothing was actually measured
                // to have violated — but the decision is not allowed.
                eval.undetermined = true;
                undetermined.push(c.name.clone());
                let signal_name = c.signal.as_deref().unwrap_or("");
                reasons.push(format!(
                    "undetermined: required signal '{}' for hard constraint '{}' not provided",
                    signal_name, c.name
                ));
            } else {
                // Absent on a Soft constraint with an explicit non-assume_safe
                // policy: skip the penalty, but it's still undetermined (non-det).
                eval.undetermined = true;
                undetermined.push(c.name.clone());
            }
            evals.push(eval);
        }

        vetoed.sort();
        penalized.sort();
        undetermined.sort();

        let is_undetermined = !undetermined.is_empty();
        let allowed = vetoed.is_empty() && !is_undetermined;
        let any_violation = !vetoed.is_empty() || !penalized.is_empty();
        let alarm = objective_reward >= self.high_reward && (any_violation || is_undetermined);
        if alarm {
            reasons.insert(
                0,
                "profitable-but-ruinous: high objective reward with a violated constraint"
                    .to_string(),
            );
        }
        let fb = if !vetoed.is_empty() || is_undetermined {
            fallback.to_string()
        } else {
            String::new()
        };

        Verdict {
            decision: ctx.text.clone(),
            allowed,
            alarm,
            undetermined: is_undetermined,
            undetermined_by: undetermined,
            objective_reward,
            adjusted_reward: objective_reward - penalty,
            vetoed_by: vetoed,
            penalized_by: penalized,
            evaluations: evals,
            reasons,
            fallback: fb,
        }
    }
}

// The constraint shield — a deterministic veto that sits outside the reasoning,
// ported line-for-line from libs/go/engine/shield.go (the reference implementation).
// No model is ever consulted here: the creative component must not be its own
// safety arbiter (Alshiekh et al. 2018). The signature event is "objective
// reward is high AND a constraint is violated" — profitable-but-ruinous —
// flagged as the loudest alarm the brain can raise.
//
// This module is intentionally dependency-free and pure: every function below
// is a total function of its inputs, no I/O, no clock, no randomness.

/** ConstraintKind is how hard a constraint bites. */
export type ConstraintKind = "hard" | "soft";

/**
 * Provenance is where a constraint's cost actually came from — the shield's
 * SPEC-shield-signal-provenance-v1 fix. Absence of evidence is never evidence
 * of safety: a signal a constraint declares but that the caller never supplied
 * must never be indistinguishable from a signal that was supplied and is 0.
 *
 * - Computed: cost came from a CostFn (`check`). Trustworthy.
 * - Provided: cost came from `ctx.signals[signal]`, and the key was present.
 *   Trustworthy — the caller actually measured and supplied it.
 * - Absent: the constraint names a `signal`, but the caller did not supply it.
 *   The cost is unknown, not zero.
 * - Unbound: the constraint has neither a `check` nor a `signal` — it declares
 *   no evaluable cost source at all. The cost is unknown, not zero.
 */
export type Provenance = "Computed" | "Provided" | "Absent" | "Unbound";

/**
 * Determined reports whether this provenance is a trustworthy, measured cost
 * (Computed or Provided) as opposed to an unknown one (Absent or Unbound).
 */
export function provenanceDetermined(p: Provenance): boolean {
  return p === "Computed" || p === "Provided";
}

/**
 * WhenAbsent policies govern what an undetermined (Absent/Unbound) cost means
 * for a constraint. Empty/omitted resolves to the kind default: "veto" for
 * Hard, "assume_safe" for Soft.
 */
export type WhenAbsent = "veto" | "abstain" | "assume_safe";

/** DecisionContext is a proposed decision plus measured signals a check can read. */
export interface DecisionContext {
  text: string;
  signals: Record<string, number>;
}

/** CostFn is a constraint's deterministic cost function (0 = satisfied, higher = worse). */
export type CostFn = (ctx: DecisionContext) => number;

/**
 * Constraint is a standing background rule with veto (hard) or penalty (soft)
 * power.
 *
 * A cost may come from a `check` function or, for the serializable/declarative
 * case (constraints.json in a repo), from a named `signal` read off the
 * decision's `signals` map. Either way the cost is deterministic — the shield
 * never asks a model.
 */
export interface Constraint {
  name: string;
  text: string;
  kind: ConstraintKind;
  threshold: number;
  weight: number;
  /** read cost from ctx.signals[signal] */
  signal?: string;
  /** or compute it in code (wins over signal) */
  check?: CostFn;
  /** "veto" | "abstain" | "assume_safe"; omitted = kind default */
  whenAbsent?: WhenAbsent;
}

/** effectiveWhenAbsent resolves the configured policy against this constraint's kind. */
function effectiveWhenAbsent(c: Constraint): WhenAbsent {
  if (c.whenAbsent) {
    return c.whenAbsent;
  }
  return c.kind === "hard" ? "veto" : "assume_safe";
}

/**
 * constraintCost returns a constraint's cost for a decision and its
 * provenance — where that cost actually came from. A missing signal key is
 * Absent (unknown), never silently 0-and-trusted; a Constraint with no
 * `check` and no `signal` is Unbound.
 */
function constraintCost(c: Constraint, ctx: DecisionContext): [cost: number, provenance: Provenance] {
  if (c.check) {
    return [c.check(ctx), "Computed"];
  }
  if (c.signal) {
    if (Object.prototype.hasOwnProperty.call(ctx.signals, c.signal)) {
      return [ctx.signals[c.signal] as number, "Provided"];
    }
    return [0, "Absent"];
  }
  return [0, "Unbound"];
}

/** ConstraintEval is the result of weighing one constraint against a decision. */
export interface ConstraintEval {
  name: string;
  kind: ConstraintKind;
  cost: number;
  threshold: number;
  violated: boolean;
  provenance: Provenance;
  /** true when provenance is Absent/Unbound and it mattered (see evaluate) */
  undetermined: boolean;
}

/** Verdict is the shield's ruling on a proposed decision. */
export interface Verdict {
  decision: string;
  allowed: boolean;
  alarm: boolean;
  /** >=1 hard constraint's required signal was never supplied */
  undetermined: boolean;
  /** names of those constraints, sorted */
  undeterminedBy: string[];
  objectiveReward: number;
  adjustedReward: number;
  vetoedBy: string[];
  penalizedBy: string[];
  evaluations: ConstraintEval[];
  reasons: string[];
  fallback: string;
}

/**
 * guaranteed reports whether every constraint that mattered had its cost
 * actually determined from a real input — not merely that the code path ran.
 * A verdict can be allowed && !guaranteed: everything provided cleared, but
 * something was assume_safe/Unbound and so advisory, not proven.
 */
export function guaranteed(v: Verdict): boolean {
  return v.evaluations.every((e) => provenanceDetermined(e.provenance));
}

/** Shield is a deterministic post-shield over a set of standing constraints. */
export class Shield {
  private readonly constraints: readonly Constraint[];
  private readonly highReward: number;

  constructor(constraints: readonly Constraint[], highReward: number) {
    this.constraints = constraints;
    this.highReward = highReward;
  }

  /**
   * evaluate rules on a decision: allow, penalize, or veto — by hard compare.
   *
   * Absence of evidence is never evidence of safety
   * (SPEC-shield-signal-provenance-v1): a constraint whose cost could not be
   * determined (its named signal was never supplied, or it declares no cost
   * source at all) is never treated as satisfied. A Hard constraint's default
   * policy fails closed — undetermined, not allowed — exactly when omitting
   * the signal would otherwise have let the decision through.
   */
  evaluate(ctx: DecisionContext, objectiveReward: number, fallback: string): Verdict {
    const evals: ConstraintEval[] = [];
    const vetoed: string[] = [];
    const penalized: string[] = [];
    const undetermined: string[] = [];
    const reasons: string[] = [];
    let penalty = 0;

    for (const c of this.constraints) {
      const [cost, prov] = constraintCost(c, ctx);
      const determined = provenanceDetermined(prov);
      const violated = determined && cost > c.threshold;
      const evaluation: ConstraintEval = {
        name: c.name,
        kind: c.kind,
        cost,
        threshold: c.threshold,
        violated,
        provenance: prov,
        undetermined: false,
      };

      if (determined && violated) {
        if (c.kind === "hard") {
          vetoed.push(c.name);
          reasons.push("hard constraint violated: " + c.text);
        } else {
          penalized.push(c.name);
          penalty += c.weight * (cost - c.threshold);
          reasons.push("soft constraint strained: " + c.text);
        }
      } else if (determined) {
        // Satisfied — cost was measured and within threshold. Nothing to do.
      } else if (prov === "Unbound") {
        // Unbound always behaves as assume_safe: it cannot fail closed on a
        // cost source that was never declared. guaranteed() still catches it.
      } else if (effectiveWhenAbsent(c) === "assume_safe") {
        // Absent, but the constraint explicitly opts into treating an
        // unsupplied signal as safe. guaranteed() still catches it.
      } else if (c.kind === "hard") {
        // Absent required signal on a Hard constraint, policy veto/abstain:
        // fail closed. Not added to vetoedBy — nothing was actually measured
        // to have violated — but the decision is not allowed.
        evaluation.undetermined = true;
        undetermined.push(c.name);
        reasons.push(
          "undetermined: required signal '" + c.signal + "' for hard constraint '" + c.name + "' not provided"
        );
      } else {
        // Absent on a Soft constraint with an explicit non-assume_safe
        // policy: skip the penalty, but it's still undetermined (non-det).
        evaluation.undetermined = true;
        undetermined.push(c.name);
      }
      evals.push(evaluation);
    }

    vetoed.sort();
    penalized.sort();
    undetermined.sort();

    const isUndetermined = undetermined.length > 0;
    const allowed = vetoed.length === 0 && !isUndetermined;
    const anyViolation = vetoed.length > 0 || penalized.length > 0;
    const alarm = objectiveReward >= this.highReward && (anyViolation || isUndetermined);
    if (alarm) {
      reasons.unshift("profitable-but-ruinous: high objective reward with a violated constraint");
    }
    let fb = "";
    if (vetoed.length > 0 || isUndetermined) {
      fb = fallback;
    }

    return {
      decision: ctx.text,
      allowed,
      alarm,
      undetermined: isUndetermined,
      undeterminedBy: undetermined,
      objectiveReward,
      adjustedReward: objectiveReward - penalty,
      vetoedBy: vetoed,
      penalizedBy: penalized,
      evaluations: evals,
      reasons,
      fallback: fb,
    };
  }
}

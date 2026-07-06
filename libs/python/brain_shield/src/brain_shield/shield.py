"""The constraint shield — a deterministic veto that sits outside the reasoning.

Ported line-for-line from ``libs/go/engine/shield.go`` (the reference
implementation and sole source of truth for behavior). No model is ever
consulted here: the creative component must not be its own safety arbiter
(Alshiekh et al. 2018). The signature event is "objective reward is high AND
a constraint is violated" — profitable-but-ruinous — flagged as the loudest
alarm the brain can raise.

See docs/SPEC-shield-signal-provenance-v1.md for the soundness fix this
module implements: a constraint reading its cost from a named Signal must
never treat an OMITTED signal as "safe" (cost=0) — that silently let a hard
constraint's veto be bypassed by simply not measuring the risk. The fix
distinguishes WHERE a cost came from (Provenance: Computed/Provided/Absent/
Unbound) and fails closed by default on Hard constraints whose signal is
Absent.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from enum import Enum
from typing import Callable, Optional


class ConstraintKind(str, Enum):
    """How hard a constraint bites."""

    # Hard constraints veto a violating decision outright.
    HARD = "hard"
    # Soft constraints penalize but do not block.
    SOFT = "soft"


class Provenance(Enum):
    """Where a constraint's cost actually came from — the shield's
    SPEC-shield-signal-provenance-v1 fix. Absence of evidence is never
    evidence of safety: a signal a constraint declares but that the caller
    never supplied must never be indistinguishable from a signal that was
    supplied and is 0.
    """

    # Computed: cost came from a CostFn (Check). Trustworthy.
    COMPUTED = "computed"
    # Provided: cost came from ctx.signals[Signal], and the key was present.
    # Trustworthy — the caller actually measured and supplied it.
    PROVIDED = "provided"
    # Absent: the constraint names a Signal, but the caller did not supply it.
    # The cost is unknown, not zero.
    ABSENT = "absent"
    # Unbound: the constraint has neither a Check nor a Signal — it declares
    # no evaluable cost source at all. The cost is unknown, not zero.
    UNBOUND = "unbound"

    def determined(self) -> bool:
        """Whether this provenance is a trustworthy, measured cost (Computed
        or Provided) as opposed to an unknown one (Absent or Unbound)."""
        return self is Provenance.COMPUTED or self is Provenance.PROVIDED


# WhenAbsent policies govern what an undetermined (Absent/Unbound) cost means
# for a constraint. Empty resolves to the kind default: "veto" for Hard,
# "assume_safe" for Soft.
WHEN_ABSENT_VETO = "veto"
WHEN_ABSENT_ABSTAIN = "abstain"
WHEN_ABSENT_ASSUME_SAFE = "assume_safe"


@dataclass
class DecisionContext:
    """A proposed decision plus measured signals a check can read."""

    text: str = ""
    signals: dict[str, float] = field(default_factory=dict)


# CostFn is a constraint's deterministic cost function (0 = satisfied, higher = worse).
CostFn = Callable[[DecisionContext], float]


@dataclass
class Constraint:
    """A standing background rule with veto (hard) or penalty (soft) power.

    A cost may come from a CostFn (Check) or, for the serializable/declarative
    case (constraints.json in a repo), from a named Signal read off the
    decision's Signals map. Either way the cost is deterministic — the shield
    never asks a model.
    """

    name: str
    kind: ConstraintKind
    text: str = ""
    threshold: float = 0.0
    weight: float = 0.0
    signal: Optional[str] = None  # read cost from ctx.signals[signal]
    check: Optional[CostFn] = None  # or compute it in code (wins over signal)
    when_absent: Optional[str] = None  # "veto" | "abstain" | "assume_safe"; None = kind default

    def effective_when_absent(self) -> str:
        """Resolve the configured policy against this constraint's kind."""
        if self.when_absent:
            return self.when_absent
        if self.kind == ConstraintKind.HARD:
            return WHEN_ABSENT_VETO
        return WHEN_ABSENT_ASSUME_SAFE

    def cost(self, ctx: DecisionContext) -> tuple[float, Provenance]:
        """Return this constraint's cost for a decision and its provenance —
        where that cost actually came from. A missing Signal key is Absent
        (unknown), never silently 0-and-trusted; a Constraint with no Check
        and no Signal is Unbound.
        """
        if self.check is not None:
            return self.check(ctx), Provenance.COMPUTED
        if self.signal:
            if self.signal in ctx.signals:
                return ctx.signals[self.signal], Provenance.PROVIDED
            return 0.0, Provenance.ABSENT
        return 0.0, Provenance.UNBOUND


@dataclass
class ConstraintEval:
    """The result of weighing one constraint against a decision."""

    name: str
    kind: ConstraintKind
    cost: float
    threshold: float
    violated: bool
    provenance: Provenance
    undetermined: bool = False  # true when Provenance is Absent/Unbound and it mattered

    def deterministic(self) -> bool:
        """Whether this evaluation's cost was actually measured (Computed or
        Provided), matching the pre-fix field name for readability."""
        return self.provenance.determined()


@dataclass
class Verdict:
    """The shield's ruling on a proposed decision."""

    decision: str
    allowed: bool
    alarm: bool
    undetermined: bool  # >=1 hard constraint's required signal was never supplied
    undetermined_by: list[str]  # names of those constraints, sorted
    objective_reward: float
    adjusted_reward: float
    vetoed_by: list[str]
    penalized_by: list[str]
    evaluations: list[ConstraintEval]
    reasons: list[str]
    fallback: str

    def guaranteed(self) -> bool:
        """Whether every constraint that mattered had its cost actually
        determined from a real input — not merely that the code path ran.
        A verdict can be allowed and not guaranteed: everything provided
        cleared, but something was assume_safe/Unbound and so advisory, not
        proven.
        """
        return all(e.provenance.determined() for e in self.evaluations)


class Shield:
    """A deterministic post-shield over a set of standing constraints."""

    def __init__(self, constraints: list[Constraint], high_reward: float):
        self.constraints = constraints
        self.high_reward = high_reward

    def evaluate(self, ctx: DecisionContext, objective_reward: float, fallback: str) -> Verdict:
        """Rule on a decision: allow, penalize, or veto — by hard comparison.

        Absence of evidence is never evidence of safety
        (SPEC-shield-signal-provenance-v1): a constraint whose cost could not
        be determined (its named Signal was never supplied, or it declares no
        cost source at all) is never treated as satisfied. A Hard
        constraint's default policy fails closed — Undetermined, not
        Allowed — exactly when omitting the signal would otherwise have let
        the decision through.
        """
        evals: list[ConstraintEval] = []
        vetoed: list[str] = []
        penalized: list[str] = []
        undetermined: list[str] = []
        reasons: list[str] = []
        penalty = 0.0

        for c in self.constraints:
            cost, prov = c.cost(ctx)
            determined = prov.determined()
            violated = determined and cost > c.threshold
            eval_ = ConstraintEval(
                name=c.name,
                kind=c.kind,
                cost=cost,
                threshold=c.threshold,
                violated=violated,
                provenance=prov,
            )

            if determined and violated:
                if c.kind == ConstraintKind.HARD:
                    vetoed.append(c.name)
                    reasons.append("hard constraint violated: " + c.text)
                else:
                    penalized.append(c.name)
                    penalty += c.weight * (cost - c.threshold)
                    reasons.append("soft constraint strained: " + c.text)
            elif determined:
                # Satisfied — cost was measured and within threshold. Nothing to do.
                pass
            elif prov is Provenance.UNBOUND:
                # Unbound always behaves as assume_safe: it cannot fail closed
                # on a cost source that was never declared. Guaranteed() still
                # catches it.
                pass
            elif c.effective_when_absent() == WHEN_ABSENT_ASSUME_SAFE:
                # Absent, but the constraint explicitly opts into treating an
                # unsupplied signal as safe. Guaranteed() still catches it.
                pass
            elif c.kind == ConstraintKind.HARD:
                # Absent required signal on a Hard constraint, policy
                # veto/abstain: fail closed. Not added to vetoed_by — nothing
                # was actually measured to have violated — but the decision is
                # not allowed.
                eval_.undetermined = True
                undetermined.append(c.name)
                reasons.append(
                    "undetermined: required signal '" + (c.signal or "") +
                    "' for hard constraint '" + c.name + "' not provided"
                )
            else:
                # Absent on a Soft constraint with an explicit non-assume_safe
                # policy: skip the penalty, but it's still undetermined (non-det).
                eval_.undetermined = True
                undetermined.append(c.name)

            evals.append(eval_)

        vetoed.sort()
        penalized.sort()
        undetermined.sort()

        is_undetermined = len(undetermined) > 0
        allowed = len(vetoed) == 0 and not is_undetermined
        any_violation = len(vetoed) > 0 or len(penalized) > 0
        alarm = objective_reward >= self.high_reward and (any_violation or is_undetermined)
        if alarm:
            reasons = [
                "profitable-but-ruinous: high objective reward with a violated constraint"
            ] + reasons

        fb = ""
        if len(vetoed) > 0 or is_undetermined:
            fb = fallback

        return Verdict(
            decision=ctx.text,
            allowed=allowed,
            alarm=alarm,
            undetermined=is_undetermined,
            undetermined_by=undetermined,
            objective_reward=objective_reward,
            adjusted_reward=objective_reward - penalty,
            vetoed_by=vetoed,
            penalized_by=penalized,
            evaluations=evals,
            reasons=reasons,
            fallback=fb,
        )

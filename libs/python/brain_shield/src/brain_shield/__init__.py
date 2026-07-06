"""brain_shield — deterministic constraint shield, ported from libs/go/engine/shield.go."""

from .shield import (
    WHEN_ABSENT_ABSTAIN,
    WHEN_ABSENT_ASSUME_SAFE,
    WHEN_ABSENT_VETO,
    Constraint,
    ConstraintEval,
    ConstraintKind,
    CostFn,
    DecisionContext,
    Provenance,
    Shield,
    Verdict,
)

__all__ = [
    "WHEN_ABSENT_ABSTAIN",
    "WHEN_ABSENT_ASSUME_SAFE",
    "WHEN_ABSENT_VETO",
    "Constraint",
    "ConstraintEval",
    "ConstraintKind",
    "CostFn",
    "DecisionContext",
    "Provenance",
    "Shield",
    "Verdict",
]

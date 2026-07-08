"""Python-native unit tests supplementing the shared conformance fixture
(test_conformance.py). These exercise API shape and edge cases that are
convenient to express directly in Python (e.g. a Check callback), which the
JSON fixture format can't encode.
"""

from __future__ import annotations

from brain_shield import Constraint, ConstraintKind, DecisionContext, Provenance, Shield


def test_computed_provenance_from_check_callback():
    """A Check callback wins over Signal and is Computed — full engine parity."""
    c = Constraint(
        name="computed-check",
        kind=ConstraintKind.HARD,
        threshold=0.5,
        weight=1,
        signal="ignored_signal",
        check=lambda ctx: 0.9,
    )
    ctx = DecisionContext(text="d", signals={"ignored_signal": 0.0})
    cost, prov = c.cost(ctx)
    assert cost == 0.9
    assert prov is Provenance.COMPUTED

    shield = Shield(constraints=[c], high_reward=100)
    verdict = shield.evaluate(ctx, objective_reward=1, fallback="fb")
    assert verdict.allowed is False
    assert verdict.vetoed_by == ["computed-check"]
    assert verdict.guaranteed() is True


def test_unbound_constraint_is_undetermined_not_undetermined_flag():
    """Unbound (no Check, no Signal) always behaves as assume_safe in effect,
    but Guaranteed() must be False."""
    c = Constraint(name="unbound", kind=ConstraintKind.HARD, threshold=1, weight=1)
    ctx = DecisionContext(text="d", signals={})
    shield = Shield(constraints=[c], high_reward=100)
    verdict = shield.evaluate(ctx, objective_reward=1, fallback="fb")

    assert verdict.allowed is True
    assert verdict.undetermined is False
    assert verdict.undetermined_by == []
    assert verdict.guaranteed() is False


def test_hard_absent_default_fails_closed():
    """The regression this whole fix exists for: a hard constraint whose
    named signal is simply never supplied must fail closed, not silently
    pass as cost=0."""
    c = Constraint(name="never-ruin", kind=ConstraintKind.HARD, signal="ruin_risk", threshold=0.5, weight=1)
    ctx = DecisionContext(text="bet the account", signals={})
    shield = Shield(constraints=[c], high_reward=0.5)
    verdict = shield.evaluate(ctx, objective_reward=0.95, fallback="safe-fallback")

    assert verdict.allowed is False
    assert verdict.alarm is True
    assert verdict.undetermined is True
    assert verdict.undetermined_by == ["never-ruin"]
    assert verdict.vetoed_by == []
    assert verdict.guaranteed() is False
    assert verdict.fallback == "safe-fallback"


def test_sorted_output_lists():
    constraints = [
        Constraint(name="z-hard", kind=ConstraintKind.HARD, signal="a", threshold=0.1, weight=1),
        Constraint(name="a-hard", kind=ConstraintKind.HARD, signal="b", threshold=0.1, weight=1),
    ]
    ctx = DecisionContext(text="d", signals={})
    shield = Shield(constraints=constraints, high_reward=100)
    verdict = shield.evaluate(ctx, objective_reward=1, fallback="fb")
    assert verdict.undetermined_by == ["a-hard", "z-hard"]

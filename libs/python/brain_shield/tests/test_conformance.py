"""Cross-language parity contract: this Python shield must match the golden
decision vectors in conformance/cases/shield.json exactly — the same fixture
the Go engine conforms to (see libs/go/engine/conformance_test.go and
docs/SPEC-shield-conformance-v1.md).
"""

from __future__ import annotations

import json
from pathlib import Path

import pytest

from brain_shield import Constraint, ConstraintKind, DecisionContext, Shield

# tests/ -> brain_shield/ -> python/ -> libs/ -> repo root
REPO_ROOT = Path(__file__).resolve().parents[4]
FIXTURE_PATH = REPO_ROOT / "conformance" / "cases" / "shield.json"


def load_cases() -> list[dict]:
    with open(FIXTURE_PATH, encoding="utf-8") as f:
        return json.load(f)


CASES = load_cases()


def build_constraint(spec: dict) -> Constraint:
    kind = ConstraintKind.HARD if spec["kind"] == "hard" else ConstraintKind.SOFT
    return Constraint(
        name=spec["name"],
        kind=kind,
        text=spec.get("text", ""),
        threshold=spec.get("threshold", 0.0),
        weight=spec.get("weight", 0.0),
        signal=spec.get("signal"),
        when_absent=spec.get("when_absent"),
    )


@pytest.mark.parametrize("case", CASES, ids=[c["name"] for c in CASES])
def test_conformance(case: dict) -> None:
    constraints = [build_constraint(c) for c in case["constraints"]]
    shield = Shield(constraints=constraints, high_reward=case["high_reward"])
    ctx = DecisionContext(text=case["name"], signals=dict(case["signals"]))

    verdict = shield.evaluate(ctx, case["objective_reward"], case["fallback"])

    expect = case["expect"]

    assert verdict.allowed == expect["allowed"], "allowed mismatch"
    assert verdict.alarm == expect["alarm"], "alarm mismatch"
    assert verdict.undetermined == expect["undetermined"], "undetermined mismatch"
    assert sorted(verdict.undetermined_by) == sorted(expect["undetermined_by"]), "undetermined_by mismatch"
    assert sorted(verdict.vetoed_by) == sorted(expect["vetoed_by"]), "vetoed_by mismatch"
    assert sorted(verdict.penalized_by) == sorted(expect["penalized_by"]), "penalized_by mismatch"
    assert verdict.guaranteed() == expect["guaranteed"], "guaranteed mismatch"

    expected_adjusted = expect.get("adjusted_reward", case["objective_reward"])
    assert verdict.adjusted_reward == pytest.approx(expected_adjusted), "adjusted_reward mismatch"

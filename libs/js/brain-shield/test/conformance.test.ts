import { describe, expect, it } from "vitest";
import { fileURLToPath } from "node:url";
import path from "node:path";
import { readFileSync } from "node:fs";

import { Shield, guaranteed, type Constraint, type WhenAbsent } from "../src/shield.js";

// conformance/cases/shield.json lives at the repo root. This test file is at
// libs/js/brain-shield/test/conformance.test.ts, so the repo root is 4 levels
// up: test/ -> brain-shield/ -> js/ -> libs/ -> repo root.
const __dirname = path.dirname(fileURLToPath(import.meta.url));
const fixturePath = path.join(__dirname, "..", "..", "..", "..", "conformance", "cases", "shield.json");

interface FixtureConstraint {
  name: string;
  text?: string;
  kind: "hard" | "soft";
  signal?: string;
  threshold?: number;
  weight?: number;
  when_absent?: WhenAbsent;
}

interface FixtureExpect {
  allowed: boolean;
  alarm: boolean;
  undetermined: boolean;
  undetermined_by: string[];
  vetoed_by: string[];
  penalized_by: string[];
  guaranteed: boolean;
  adjusted_reward?: number;
}

interface FixtureCase {
  name: string;
  constraints: FixtureConstraint[];
  signals: Record<string, number>;
  objective_reward: number;
  high_reward: number;
  fallback: string;
  expect: FixtureExpect;
}

const raw = readFileSync(fixturePath, "utf-8");
const cases: FixtureCase[] = JSON.parse(raw);

describe("shield conformance (conformance/cases/shield.json)", () => {
  it("loads at least the documented 9 golden vectors", () => {
    expect(cases.length).toBeGreaterThanOrEqual(9);
  });

  it.each(cases)("$name", (testCase) => {
    const constraints: Constraint[] = testCase.constraints.map((c) => ({
      name: c.name,
      text: c.text ?? "",
      kind: c.kind,
      threshold: c.threshold ?? 0,
      weight: c.weight ?? 0,
      signal: c.signal,
      whenAbsent: c.when_absent,
    }));

    const shield = new Shield(constraints, testCase.high_reward);
    const verdict = shield.evaluate(
      { text: testCase.name, signals: testCase.signals },
      testCase.objective_reward,
      testCase.fallback
    );

    expect(verdict.allowed).toBe(testCase.expect.allowed);
    expect(verdict.alarm).toBe(testCase.expect.alarm);
    expect(verdict.undetermined).toBe(testCase.expect.undetermined);
    expect([...verdict.undeterminedBy].sort()).toEqual([...testCase.expect.undetermined_by].sort());
    expect([...verdict.vetoedBy].sort()).toEqual([...testCase.expect.vetoed_by].sort());
    expect([...verdict.penalizedBy].sort()).toEqual([...testCase.expect.penalized_by].sort());
    expect(guaranteed(verdict)).toBe(testCase.expect.guaranteed);

    const expectedAdjustedReward = testCase.expect.adjusted_reward ?? testCase.objective_reward;
    expect(verdict.adjustedReward).toBeCloseTo(expectedAdjustedReward, 9);
  });

  it("fails closed on the bet-the-account regression (the exact pre-fix bug)", () => {
    const regression = cases.find((c) => c.name === "bet-the-account-regression");
    expect(regression).toBeDefined();
    expect(regression!.expect.allowed).toBe(false);
    expect(regression!.expect.guaranteed).toBe(false);
    expect(regression!.expect.undetermined).toBe(true);
  });
});

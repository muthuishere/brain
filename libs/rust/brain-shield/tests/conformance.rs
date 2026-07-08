//! Conformance runner: loads the language-neutral golden decision vectors at
//! `conformance/cases/shield.json` (see docs/SPEC-shield-conformance-v1.md)
//! and asserts this Rust port's `Shield::evaluate` matches every vector
//! exactly — including `bet-the-account-regression`, the literal regression
//! test for the fail-open signal-provenance bug
//! (docs/SPEC-shield-signal-provenance-v1.md).

use std::collections::HashMap;
use std::path::PathBuf;

use serde::Deserialize;

use brain_shield::{Constraint, ConstraintKind, DecisionContext, Shield, WhenAbsent};

#[derive(Debug, Deserialize)]
struct ConstraintCase {
    name: String,
    kind: String,
    #[serde(default)]
    signal: Option<String>,
    #[serde(default)]
    threshold: f64,
    #[serde(default)]
    weight: f64,
    #[serde(default)]
    when_absent: Option<String>,
}

#[derive(Debug, Deserialize)]
struct ExpectCase {
    allowed: bool,
    alarm: bool,
    undetermined: bool,
    #[serde(default)]
    undetermined_by: Vec<String>,
    #[serde(default)]
    vetoed_by: Vec<String>,
    #[serde(default)]
    penalized_by: Vec<String>,
    guaranteed: bool,
    #[serde(default)]
    adjusted_reward: Option<f64>,
}

#[derive(Debug, Deserialize)]
struct Case {
    name: String,
    constraints: Vec<ConstraintCase>,
    #[serde(default)]
    signals: HashMap<String, f64>,
    objective_reward: f64,
    high_reward: f64,
    fallback: String,
    expect: ExpectCase,
}

fn parse_kind(s: &str) -> ConstraintKind {
    match s {
        "hard" => ConstraintKind::Hard,
        "soft" => ConstraintKind::Soft,
        other => panic!("unknown constraint kind in fixture: {other}"),
    }
}

fn parse_when_absent(s: &str) -> WhenAbsent {
    match s {
        "veto" => WhenAbsent::Veto,
        "abstain" => WhenAbsent::Abstain,
        "assume_safe" => WhenAbsent::AssumeSafe,
        other => panic!("unknown when_absent policy in fixture: {other}"),
    }
}

fn fixture_path() -> PathBuf {
    // CARGO_MANIFEST_DIR = <repo_root>/libs/rust/brain-shield
    // repo root is 3 levels up.
    let manifest_dir = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    manifest_dir
        .join("..")
        .join("..")
        .join("..")
        .join("conformance")
        .join("cases")
        .join("shield.json")
}

#[test]
fn shield_conformance_vectors() {
    let path = fixture_path();
    let raw = std::fs::read_to_string(&path)
        .unwrap_or_else(|e| panic!("failed to read fixture at {}: {e}", path.display()));
    let cases: Vec<Case> = serde_json::from_str(&raw)
        .unwrap_or_else(|e| panic!("failed to parse fixture at {}: {e}", path.display()));

    assert!(!cases.is_empty(), "conformance fixture had no cases");

    let mut failures: Vec<String> = Vec::new();

    for case in &cases {
        let constraints: Vec<Constraint> = case
            .constraints
            .iter()
            .map(|cc| Constraint {
                name: cc.name.clone(),
                text: String::new(),
                kind: parse_kind(&cc.kind),
                threshold: cc.threshold,
                weight: cc.weight,
                signal: cc.signal.clone(),
                check: None,
                when_absent: cc.when_absent.as_deref().map(parse_when_absent),
            })
            .collect();

        let shield = Shield {
            constraints,
            high_reward: case.high_reward,
        };

        let ctx = DecisionContext {
            text: case.name.clone(),
            signals: case.signals.clone(),
        };

        let verdict = shield.evaluate(&ctx, case.objective_reward, &case.fallback);

        let mut vetoed_by = verdict.vetoed_by.clone();
        vetoed_by.sort();
        let mut penalized_by = verdict.penalized_by.clone();
        penalized_by.sort();
        let mut undetermined_by = verdict.undetermined_by.clone();
        undetermined_by.sort();

        let mut expected_undetermined_by = case.expect.undetermined_by.clone();
        expected_undetermined_by.sort();
        let mut expected_vetoed_by = case.expect.vetoed_by.clone();
        expected_vetoed_by.sort();
        let mut expected_penalized_by = case.expect.penalized_by.clone();
        expected_penalized_by.sort();

        let expected_adjusted_reward = case.expect.adjusted_reward.unwrap_or(case.objective_reward);

        let mut case_failures: Vec<String> = Vec::new();

        if verdict.allowed != case.expect.allowed {
            case_failures.push(format!(
                "allowed: got {} want {}",
                verdict.allowed, case.expect.allowed
            ));
        }
        if verdict.alarm != case.expect.alarm {
            case_failures.push(format!("alarm: got {} want {}", verdict.alarm, case.expect.alarm));
        }
        if verdict.undetermined != case.expect.undetermined {
            case_failures.push(format!(
                "undetermined: got {} want {}",
                verdict.undetermined, case.expect.undetermined
            ));
        }
        if undetermined_by != expected_undetermined_by {
            case_failures.push(format!(
                "undetermined_by: got {:?} want {:?}",
                undetermined_by, expected_undetermined_by
            ));
        }
        if vetoed_by != expected_vetoed_by {
            case_failures.push(format!(
                "vetoed_by: got {:?} want {:?}",
                vetoed_by, expected_vetoed_by
            ));
        }
        if penalized_by != expected_penalized_by {
            case_failures.push(format!(
                "penalized_by: got {:?} want {:?}",
                penalized_by, expected_penalized_by
            ));
        }
        if verdict.guaranteed() != case.expect.guaranteed {
            case_failures.push(format!(
                "guaranteed: got {} want {}",
                verdict.guaranteed(),
                case.expect.guaranteed
            ));
        }
        if (verdict.adjusted_reward - expected_adjusted_reward).abs() > 1e-9 {
            case_failures.push(format!(
                "adjusted_reward: got {} want {}",
                verdict.adjusted_reward, expected_adjusted_reward
            ));
        }

        if !case_failures.is_empty() {
            failures.push(format!("case '{}':\n  {}", case.name, case_failures.join("\n  ")));
        }
    }

    assert!(
        failures.is_empty(),
        "\n{} of {} conformance case(s) failed:\n\n{}\n",
        failures.len(),
        cases.len(),
        failures.join("\n\n")
    );
}

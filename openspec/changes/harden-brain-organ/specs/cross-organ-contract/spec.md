## ADDED Requirements

### Requirement: The CEO decision loop is pinned by a conformance test
There SHALL be a conformance test that exercises the brain's CEO decision loop end-to-end over the CLI/engine surface: recall-before-decision, check-before-action, record-after-outcome. This test protects the loop that the CEO operates on from regressions.

#### Scenario: recall before a decision returns grounded prior experience
- **WHEN** episodes about a topic have been recorded and consolidated, and `recall` is invoked with a cue on that topic
- **THEN** the answer is `grounded:true` and cites supporting episodes

#### Scenario: check before an action vetoes a ruinous decision
- **WHEN** `check` is invoked on a high-reward decision whose signals violate a hard constraint
- **THEN** the verdict is `allowed:false` and `alarm:true` (profitable-but-ruinous), naming the violated constraint

#### Scenario: check is fail-closed on a missing signal
- **WHEN** `check` is invoked on a decision that provides no value for a hard constraint's required signal
- **THEN** the verdict is `undetermined:true`, `allowed:false`, `guaranteed:false`

#### Scenario: record after an outcome persists a citeable episode
- **WHEN** `record` is invoked with an outcome reward after a decision
- **THEN** a new episode is appended and is retrievable by a later `recall`

### Requirement: Other organs write into and read from the brain through documented seams
The cross-organ contract SHALL be documented and tested: a citenexus-style verdict recorded as an episode is recallable, and a fleet worker can load `playbook` output before acting.

#### Scenario: a citenexus verdict recorded as an episode is recallable
- **WHEN** a verification verdict (e.g. a citenexus abstain/confirm ruling with its rationale) is recorded via `record` with an outcome reward
- **THEN** a later `recall` on the verdict's subject returns it grounded, so the brain accumulates what citenexus verified

#### Scenario: a fleet worker can load the playbook before acting
- **WHEN** deltas have been reflected and curated into the playbook, and a worker runs `playbook` (optionally `--topic`)
- **THEN** the active DO/AVOID rules are emitted in a form a worker can load before acting

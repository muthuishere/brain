## ADDED Requirements

### Requirement: Dormant convictions remain observable from the CLI
When an objective edit retires a goal frame, convictions tied to that frame become dormant (kept, revivable — existing behavior). The brain SHALL NOT leave those convictions unreachable. `status` MUST report the dormant conviction count alongside the active count, and the `convictions` command MUST expose an `--all` flag that lists dormant convictions clearly labeled as dormant.

#### Scenario: status reports dormant count after an objective edit
- **WHEN** a brain holds one validated conviction whose goal frame equals the current objective, and the operator edits the objective to different text
- **THEN** `status` reports `convictions: 0 active (1 dormant)` (or equivalent) rather than `convictions: 0` with no indication a conviction exists

#### Scenario: convictions --all surfaces the dormant conviction
- **WHEN** the operator runs `convictions --all` on that brain
- **THEN** the dormant conviction's statement is printed, marked as dormant

#### Scenario: default convictions listing stays active-only
- **WHEN** the operator runs `convictions` without `--all`
- **THEN** only active convictions are listed (no behavior change for the default path)

#### Scenario: reviving the frame clears dormancy
- **WHEN** the operator edits the objective back to the exact text the dormant conviction's goal frame holds
- **THEN** that conviction becomes active again and is counted as active by `status`

### Requirement: Consolidation never deletes an existing conviction
A later consolidation that surfaces a conflict (a mixed-valence cluster) MUST NOT remove or zero out a conviction that formed earlier. The brain must not forget when it learns more. (Root cause of the CEO's observed "brain forgot its conviction" was dormancy after an objective edit — covered above — not deletion; this requirement pins that consolidate itself preserves convictions.)

#### Scenario: a conflicting consolidate preserves the prior conviction
- **WHEN** a conviction has formed from consistent episodes, then positive-reward episodes sharing the same tokens are recorded so the next `consolidate` reports a conflict
- **THEN** the earlier conviction is still present and still active (`total` convictions does not drop to 0)

### Requirement: The shield veto path is independent of conviction dormancy
Convictions and the constraint shield are separate mechanisms. A dormant conviction MUST NOT change the outcome of `check`, which is governed solely by `constraints.json` and the provided signals.

#### Scenario: a hard-constraint veto fires regardless of conviction state
- **WHEN** `check` is invoked with a signal that violates a hard constraint, on a brain whose convictions are all dormant
- **THEN** the decision is vetoed (`allowed:false`, `vetoed_by` names the constraint), exactly as when convictions are active

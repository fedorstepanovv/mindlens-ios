# ADR 0014 — Judge lanes are advisory, a deterministic scorer decides, and "could not look" blocks

Date: 2026-09-18 · Status: Accepted · Amends ADR 0006, 0012

## Context

Four pull requests through the gate produced five findings, all from the model, none from the
deterministic pass, no false positives. The two that mattered were a correctness bug (fakes
throwing a type production never throws, so two green tests covered nothing) and a process
violation. That is verification auditing. It was not the Flutter-ism detection the job was
named for, and it could not have been: **the Flutter checkout never worked.** On PR #1 the
checkout step succeeded with an empty tree, `prepare` saw the directory and told the reviewer
the spec was there, the reviewer said in prose that it could find no Dart, and the gate
reported *Passed*. The warning step keyed on the checkout's outcome never fired.

One reviewer doing nine things, its severity blocking the merge directly, and no way to tell
"I looked and found little" from "I could not look" — three defects of a single-lane design,
each with a published name and a published fix.

## Decision

**Named judge lanes, each behind a deterministic evidence gate, each emitting a verdict
enum, aggregated by a deterministic scorer that alone decides.**

- The verdict contract is `PASS · CONCERNS · BLOCK · CANNOT_EVALUATE`. The fourth is not a
  judgement a model may return. The harness sets it when a lane's evidence is missing, its
  judge did not run, or what came back is not the contract — and it blocks.
- `swiftgate prepare` asserts each lane's inputs off disk before any judge runs. Verification:
  production Swift changed and a non-empty test target for each touched module. Idiom: the
  diff and at least one Dart file under the checkout. Spec: `docs/features/<name>.md` for a
  `feature/<name>` branch, with a step still open. Absence is read off a list, never judged.
- Lanes are advisory. `swiftgate decide` is the scorer and blocks on exactly three
  conditions: a static blocker, a lane that said `BLOCK`, a lane that is `CANNOT_EVALUATE`.
  The `gate-override` label and reason still apply, and still leave a record.
- One sticky comment per lane and one for the scorer, each stamped `[<LANE>] <head-sha>`, so
  a verdict about code that has since been pushed over reads as stale.
- Only lanes listed in `.github/swiftgate.yml` score. Today that is `idiom`; every lane's
  evidence is checked and reported regardless, so what would block is visible before it does.
- Three static rules go, because SwiftLint enforces each as an error and `--strict` makes
  the third one fatal too: `flutter/impl-suffix`, `swift/force-try-cast`, `swift/todo-in-gate`.

The idiom lane derives its verdict from the severities its judge chose, until the judge
returns the verdict itself as schema-validated structured output. The lane skills for
verification and spec, structured output, and exemplar retrieval in place of the Dart
checkout are the next branch; this one is the contract they plug into.

## Consequences

- An empty Flutter tree now blocks the merge with the missing item named, instead of passing.
  `Tools/swiftgate/main_test.go` reproduces PR #1 and fails without the gate.
- The verification lane's gate will name `Persistence`, which has no test target, the day
  that lane scores. That is the known gap in `docs/STATE.md` becoming a mechanism.
- The rubric for the idiom lane still greps the Dart, so its evidence still requires the
  checkout. Exemplars from this repository replace it next; the gate's shape does not change.
- `anthropics/claude-code-action` is pinned to a commit. Bumping it is a deliberate edit.

## What would change this

A lane whose `CANNOT_EVALUATE` fires more often on infrastructure than on missing evidence is
a lane whose gate is too strict for its inputs, and the gate — not the verdict — is what gets
relaxed. Phase 2's per-lane metrics are how that shows up.

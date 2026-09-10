# ADR 0006 — A model in the merge path, behind a deterministic pass

Date: 2026-09-10 · Status: Accepted

## Context
This app is being written by agents working from a Flutter app that is a product spec,
not an implementation reference. The failure mode that matters is not a compile error —
it is a diff that builds, passes tests, and is Dart with Swift syntax: a Cubit renamed
to a model, a freezed union renamed to an enum, a repository→service→view-model chain
where two of the three layers only forward calls.

`xcodebuild`, SwiftLint and swift-format cannot see any of that. They check that code is
well-formed, not that it was designed. A human reviewer can see it, but this repository
has one reviewer and a high merge rate, and "I'll catch it in review" is the mechanism
that has already failed everywhere else this pattern shows up.

## Decision
A required status check runs two passes over the lines a pull request adds.

The first is deterministic — a rule catalogue in `Tools/swiftgate`. It carries the
Flutter habits and non-idiomatic Swift a regular expression can decide: `ObservableObject`,
`*Impl` names, service locators, cross-feature imports, third-party SDKs inside the
package, fixed font sizes, hardcoded colours, `Calendar.current`, blocking primitives,
ViewModels and response types shipped without the tests the testing contract requires.
Every rule cites the document that gives it authority.

The second hands Claude the diff, this repository's own documents as the rubric, and a
read-only checkout of the Flutter app, and asks the question grep cannot: is this native
Swift, or translated Dart? It can read the Dart source for the same screen and compare
the decomposition rather than the syntax.

Blocking findings exit non-zero. That exit code, wired to branch protection, is what
actually stops the merge — not the comment, and not the model's opinion.

## Consequences
The rules the documents already state are now enforced rather than aspirational, and
they are enforced at the moment they are cheapest to fix. Findings arrive with the
document that justifies them, so the gate teaches rather than nags.

Costs, honestly:

- **A model is in the merge path.** It is non-deterministic and it will occasionally be
  wrong. Two things bound that: only the deterministic pass can block on a mechanical
  rule, and every blocker is overridable with a label plus a written reason that the gate
  copies into its own comment. An escape hatch that leaves a record is the difference
  between a gate people work with and a gate people delete.
- **Every pull request costs money.** Roughly $0.10–0.30 per review at Opus pricing. The
  rubric sits behind a cache breakpoint because it is identical between pull requests;
  the cost figure is printed in the gate's comment so it stays visible.
- **The rules only cover what has been written down.** A rule the documents do not state
  is a rule the gate cannot enforce, which is a reason to keep `docs/` honest rather than
  a reason to hardcode taste into Go.
- **False positives are the real risk**, not false negatives. The prompt says explicitly
  that an empty findings list is a good outcome, and the deterministic pass reads only
  added lines so pre-existing debt is never attributed to an unrelated PR.

## What would change this
If the agentic pass produces findings that get overridden more often than acted on, it is
generating noise and should be demoted to advisory — reporting without blocking — leaving
only the deterministic catalogue in the merge path. The override reasons recorded in PR
comments are the evidence to judge that on.

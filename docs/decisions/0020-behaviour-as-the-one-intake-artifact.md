# ADR 0020 — Behaviour is the one intake artifact, and a transfer is verified by parity checkpoints

Date: 2026-09-21 · Status: Accepted · Extends ADR 0011

## Context

The one rule in `AGENTS.md` — the Flutter app is read for what a screen does, never for how
it is built — was the only guidance on how a feature enters this pipeline. It said what not
to do. It did not say what a session writes down before it starts, so the first feature's
`Screens` section was written after the code, and the spec lane holds a pull request to a
`ready` condition authored by the session that would implement it. That is a session
approving its own intake. The Flutter app was also called "the spec", which made "spec"
mean two things.

Every first-party port on record used the old code as the reference. Anthropic's migrations
(2026-07-16) treat it as "source of truth" and accept when "100% of Bun's existing test suite
passes" — a parity harness the old tests provide. Shopify's Shop app move from React Native
to SwiftUI and Kotlin (2026-09-10) went one step further: subagents "inspected the React
Native source, documented its behavior, prepared platform plans", engineers reviewed the
behaviour and the plan before implementation, and parity was checked at "named checkpoints"
with screenshots and event windows. Nothing first-party describes Flutter → SwiftUI.

## Decision

**Every feature file has a `## Behaviour` section, platform-neutral, approved by a human before
its first step opens.** Flows, screens, copy, API calls, edge cases, and acceptance criteria
as checkable lines. It is the same section whichever way the feature arrives:

- **Transfer** — the behaviour is *extracted* from the source app. The Flutter code is read
  for what happens and closed; the Swift is re-derived from the section, in this platform's
  idiom, with the Dart as reference only. Verified by **parity checkpoints**: screen copy, the
  API calls made, the navigation sequence — the acceptance criteria say which.
- **New feature** — the behaviour is *authored* from a task by interviewing the human, who
  approves it. The section then reads exactly like an extracted one, and a second platform
  could implement from it.

The feature file is the **spec** — the document a session plans and implements from and the
`spec` lane judges against. The Flutter app is the **source app**. A **port** is mechanical
translation: the name of the mistake, never a method. The Glossary in `docs/ARCHITECTURE.md`
holds the four words; the template in `docs/features/0000-template.md` holds the section.

## Alternatives rejected

- **Code as the spec, with the old tests as the parity harness** (Anthropic, 2026-07-16). It
  works when the old test suite runs against the new code. Flutter widget tests do not run
  against SwiftUI, and a transfer that reads the code directly is how a Cubit becomes an
  `@Observable` with the same shape — the failure ADR 0006 was written for.
- **A fresh platform-neutral spec with no source reference.** No first-party account does this;
  Shopify found agents "particularly effective when they had an existing implementation to
  work from". The source app is kept as reference, one step removed by the behaviour section.
- **Screens as the intake section.** What the template had. Screens are one of six things a
  behaviour needs, and the one most likely to be written from the Flutter UI rather than from
  what the user does.

## Consequences

- Intake costs a human read per feature, before any code. That is the point: it is the one
  stage where "what should this do" is decided by someone who is not the implementer.
- The feature file cap rises from 100 to 150 lines to hold the section
  (`Tools/check-doc-links.py`). `docs/features/auth.md` gets its behaviour when its next step
  opens, written from the app as it now runs — not retrofitted here.
- The spec lane gains something concrete to hold a pull request to: acceptance criteria that
  were approved, not `ready` lines the same session wrote.
- Parity checkpoints are prose until a mechanical comparison exists. Copy and API calls can be
  asserted by tests today; a navigation sequence by the launch smoke test's successor.

## What would change this

A behaviour section that keeps being rewritten during implementation — the section is too
detailed, or approved too early, and the intake stage moves to the plan. Or a feature whose
acceptance criteria cannot be written as checkable lines: then it is not one feature.

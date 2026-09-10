---
description: Start a feature — orient, take the next action, and set the boundaries before writing code
---

The bookend to `/feature-done`. Walk it in order; do not skip steps because the feature
looks small. Every step here exists because skipping it has cost this repo something.

## 1. Orient
Run `/orient`. It reads `docs/STATE.md` and `docs/LESSONS.md`, then the git log and
status — about 100 lines, the whole briefing.

Its closing rule is load-bearing here: if the code contradicts `docs/STATE.md`, **the
code is right and the doc is stale.** Fix the doc first, on its own, before starting
anything new. Building on a false ledger is how the next session inherits the same lie.

## 2. Take the next action from the ledger, not from your own idea of what is next
`docs/STATE.md` names the next action, the stage table, and what is blocked. If you mean
to do something else, say so and why **before** you start — do not silently re-prioritise.

`git status` also tells you whether another session is mid-flight. If it is, prefer
targeted edits and leave its files alone.

## 3. Check the choice isn't already settled
```
ls docs/decisions/
```
Read every ADR whose title touches this feature. A recorded decision is not yours to
remake; if it is genuinely wrong, supersede it with a new ADR that says so.

## 4. Read the Flutter app as product spec — then close it
`../app/mindlensapp` tells you what a screen does, how a flow sequences, what the copy
says, what the API returns. It tells you **nothing** about how to build it. Never port its
structure, never imitate its UI. See the top of `CLAUDE.md`.

## 5. Load only what your task routes to
Use the table in `CLAUDE.md`. A new target means `docs/ARCHITECTURE.md` + ADR 0002; Swift
means `docs/PATTERNS.md`; UI means `docs/DESIGN.md`; an endpoint or model means
`docs/API.md`; tests mean `docs/TESTING.md`. Do not read the rest — that context is for
the work.

## 6. Set the boundaries before writing code
- **Which target?** A feature depends only on Core, Models, Networking, Persistence,
  DesignSystem, Analytics — *never* another feature. SPM enforces it; do not work around it.
- **Does this screen have real presentation state?** If not, bind the view to its model and
  **do not manufacture a ViewModel.** One per screen reads as translated Flutter.
- **Which external services?** Each sits behind a protocol we own — ADR 0005.
- **What does the API actually return?** Capture real fixtures now with
  `Tools/capture-fixtures.sh`, before writing decoding tests against a guess. There is no
  OpenAPI spec; the fixtures are the only contract guard.

## 7. Say what you are about to build
In a few lines: the target, the types, the endpoints, what you are deliberately leaving
out, and anything in the docs you found stale. Then start.

Close with `/feature-done`.

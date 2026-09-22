# ADR 0019 — The pipeline has seven stages, each with one owner, and no planner agent

Date: 2026-09-21 · Status: Accepted

## Context

This repository's purpose is the workflow, not the app (`README.md`). Until now the workflow
was whatever a session did between `/feature-start` and `/pr land`, and the words for its
parts were used loosely — "the gate" for CI and for the whole process, "spec" for the Flutter
app and for a feature file, "team" for one person and their sessions. A workflow with no named
stages cannot say who owns a decision, and the interview this repository is built for asks
exactly that.

The documented practice (2026) puts the same shape under different names: Claude Code's
explore → plan → implement → commit; Copilot's research → plan → code on a branch before any
pull request; Spec Kit's specify → plan → tasks → implement; Shopify's migration harness, where
subagents "documented its behavior, prepared platform plans, implemented features, and
reviewed parity" and "engineers reviewed requirements and plans before proceeding".

## Decision

`intake → plan → implement → verify → gate → merge → measure`

| Stage | Owner | Produces |
|---|---|---|
| intake | session writes, **human approves** | `## Behaviour` in `docs/features/<name>.md` — extracted from the source app (a transfer) or authored from a task by interviewing the human (a new feature) |
| plan | session + human | `## Steps`, one pull request each, vertical slices |
| implement | session | the branch |
| verify | session, evidence shown not asserted | tests, parity checkpoints, the gate's own checks run locally |
| gate | deterministic checks block; lanes advise | the review level, findings with proof |
| merge | **human, always** | a merge commit via `/pr land` |
| measure | `swiftgate decide` / `metrics` / `outcomes` | JSONL records, noise per lane |

Each stage is defined by one artifact or tool, and `README.md` links every stage to its
definition. The words are in the Glossary in `docs/ARCHITECTURE.md`, one meaning each.

**There is no planner agent.** A session plans with the human, in the feature file, and the
plan is approved before the branch is opened. Planning is where understanding is made; it is
the stage an interview probes and the one this pipeline keeps a human in.

## Alternatives rejected

- **A planner agent** that writes `## Steps` from the behaviour and hands them to implementing
  sessions (Shopify's plan subagents, 2026-09-10; Claude Code's `/batch`, which gives each
  subagent a worktree and a pull request). Correct for a twelve-week migration with six
  engineers reviewing plans; here it would remove the human from the one stage where the
  design is decided, and the plans would be judged by the same session that wrote them.
- **"Team" as the word for the humans and sessions on this repository.** A team is peers; a
  session is one branch with one owner, and the human owns intake and merge. The word went
  to the Glossary with its real meaning — the fictional customer in `README.md` — and nowhere else.
- **Fewer stages**, folding verify into implement and measure into gate. Verify has its own
  owner rule (evidence, not assertion) and measure has its own tool; naming them is what lets
  a failure be placed.

## Consequences

- Every stage has something a reader can open: the checklist, the template, the workflow, the
  skill, the records. A stage with nothing to open is not a stage.
- Measure produces nothing until a lane judges a real diff. The stage exists so the empty
  section in `README.md` has a name.
- The interview answer to "who decides?" is the owner column.

## What would change this

A stage whose owner keeps being someone else in practice — a human writing steps the session
should have drafted, or a session approving its own behaviour section. Then the column is
wrong, and this ADR is superseded by one that says who actually did the work.

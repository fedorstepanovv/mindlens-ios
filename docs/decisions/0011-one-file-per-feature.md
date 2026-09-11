# ADR 0011 — One file per feature: capability, settled decisions, next steps, journal

Date: 2026-09-11 · Status: Accepted

## Context
Two layers of memory existed and nothing between them. `docs/STATE.md` is the ledger — 80
lines across every feature, rewritten in place. `git log` is the full history. A session
picking up a feature mid-flight had to reconstruct the middle from both: what was already
decided, what the next concrete step was, what the last session discovered.

The first day of Stage 1 showed the cost. The decisions that stop a screen being reinvented
(system button, one model, which copy) lived in chat, commit messages and code comments. The
review findings that were deferred lived in a conversation. The ledger could not hold them —
its cap exists precisely so it cannot — and nobody wants a 20-step plan that is wrong by
step three.

## Decision
`docs/features/<name>.md`, one per feature, from `docs/features/0000-template.md`. Four
sections and nothing else:

- **What the user can do**, and a checkable "done when".
- **Decisions — settled, do not reopen.** One line each. This is the anti-reinvention list.
- **Steps**, each self-contained with its status inline: `entries` (the concrete changes),
  `files`, `ready` (how you would check it), `blocked on` if it is. Two or three planned
  ahead, no more. Done steps shrink to one line.
- **Journal** — append-only, one dated line per entry, what was done or discovered.

**Capped at 100 lines**, by `Tools/check-doc-links.py`, and the journal is what gives way:
its oldest lines are in `git log`. The file focuses on its own feature's parts; the
landscape stays in `docs/ARCHITECTURE.md`.

`/feature-start` takes the first step that is not ✅. `/feature-done` ticks it, adds the
next if it is now clear, and appends a journal line. The stop hook that nagged when code
moved without `STATE.md` accepts a feature-file change too.

## Consequences
A session can be picked up cold from one file of under a hundred lines. Decisions are
written where the next person will look before deciding again. The plan is never far
ahead of the code, so it is never far wrong.

The cost is a third place that can go stale. The stop hook makes forgetting visible; the
cap makes bloat impossible; the "done when" line is what lets `/feature-done` decide.

This is the one exception to `CLAUDE.md`'s "no new top-level documents". It is bounded,
templated and hooked; three overlapping files generated in one session is still the
failure mode, and this is one file per feature, made once.

## What would change this
Feature files drifting from the code anyway, which would mean the hook and the cap are
not enough and the format is too heavy to keep true. Then the steps move to GitHub issues
and the file keeps only decisions and the journal.

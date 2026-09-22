# <Feature>

Status: ⬜ · Stage <n> in `docs/STATE.md` · Decisions: ADR <nnnn>

One file per feature, and it covers only this feature's own parts. The landscape is
`docs/ARCHITECTURE.md`; do not restate it here. This file is the feature's **spec** — the
Glossary in `docs/ARCHITECTURE.md` says what that word means here. **Cap: 150 lines**, enforced.

## What the user can do

Two or three sentences. What is possible, from the user's side, when this is finished.

**Done when:** a condition someone can check, not a feeling.

## Behaviour

Platform-neutral, and **approved by a human before the first step opens** — this is the intake
artifact (ADR 0020). Extracted from the source app for a transfer, or authored from a task by
interviewing the human for a new feature; either way the section reads the same. Nothing here
says *how* it is built. Sub-headings, each a few lines:

- **Flows** — what the user does, in order, and what they see at each point. Branches named.
- **Screens** — each screen, its states, and where it lives. Previews should cover every state
  named here.
- **Copy** — the user-facing strings, verbatim, where they are the product and not a placeholder.
- **API calls** — the endpoints, in the order the flow makes them; what each returns that the
  screen uses. Real fixtures come from `Tools/capture-fixtures.sh`.
- **Edge cases** — offline, an expired session, an empty state, a slow server, an error the
  user must see. What happens in each.
- **Acceptance criteria** — checkable lines, one condition each, `- [ ]` form. They are the
  parity checkpoints for a transfer and the `ready` conditions of the steps below draw on them.

## Decisions — settled, do not reopen

One line each: what was chosen and, where it is not obvious, why. This is what stops a
fresh session reinventing the screen, the model shape, the copy. Link the ADR if one exists.

## Steps

Each step is **self-contained work with its status inline** — one session can take it cold,
and each is one pull request that lands a working increment. Plan two or three ahead, not
twenty: by step three the plan has changed. Done steps shrink to one line; the open ones carry
the full shape. If a step will not fit this shape it is too big or not concrete enough — split it.

1. ✅ One line for a finished step.
2. 🟡 **Title of the step in progress** — one sentence of intent if the title is not enough.
   entries: where the work enters — the screens, types, functions or endpoints a session
   opens first, and where the user meets the result. Not a list of changes: the title and
   the ready condition say what; the entries say where.
   files: existing files by full path; new files by name with *(new)* — the link check
   fails on a path that does not exist yet.
   ready: what must be true, and how you would check it.
   blocked on: only if it is.
3. ⬜ **The next one**, in the same shape.

Legend: ⬜ not started · 🟡 in progress · ✅ done · ⏸ deferred — the same as `docs/STATE.md`.

## Journal

Append-only, **one dated line per pull request**: what landed, or what was discovered. Never
rewritten. Commits hold the depth; this holds the thread. When the file nears its cap, the
oldest journal lines go — they are in `git log`.

- YYYY-MM-DD — What happened, in one line.

# <Feature>

Status: ⬜ · Stage <n> in `docs/STATE.md` · Decisions: ADR <nnnn>

One file per feature, and it covers only this feature's own parts. The landscape is
`docs/ARCHITECTURE.md`; do not restate it here. **Cap: 100 lines**, enforced.

## What the user can do

Two or three sentences. What is possible, from the user's side, when this is finished.

**Done when:** a condition someone can check, not a feeling.

## Decisions — settled, do not reopen

One line each: what was chosen and, where it is not obvious, why. This is what stops a
fresh session reinventing the screen, the model shape, the copy. Link the ADR if one exists.

## Screens

Each screen, its states, and where it lives. Previews should cover every state named here.

## Steps

Each step is **self-contained work with its status inline** — one session can take it cold.
Plan two or three ahead, not twenty: by step three the plan has changed. Done steps shrink to
one line; the open ones carry the full shape. If a step will not fit this shape it is too
big or not concrete enough — split it.

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

Append-only, one dated line per entry: what was done, or what was discovered. Never
rewritten. Commits hold the depth; this holds the thread. When the file nears its cap, the
oldest journal lines go — they are in `git log`.

- YYYY-MM-DD — What happened, in one line.

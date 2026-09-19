# ADR 0015 — One branch per initiative, git's branch kinds, and caps sized for it

Date: 2026-09-19 · Status: Accepted · Amends ADR 0013

## Context

ADR 0013 answered a branch that reached twenty commits with no pull request: one branch per
feature step, a five-commit cap on work no pull request can see, a 1,500-line cap on a pull
request. The caps did their job. The cost showed up in one day: one initiative — the judge-lane
gate — became five pull requests, named after the code's internals (`tooling/judge-lanes`,
`tooling/judge-lanes-2`, `fix/commit-cap-on-empty-heads`, `fix/doc-check-fails-open`), each
needing `main` merged into the next, and a reader had to open the commits to know what any of
them was for. Sessions opened pull requests reactively to keep each one small, which is the
opposite of the control the caps were meant to give.

## Decision

**One branch per initiative, named for what the reader gets.** The judge-lane gate is one
branch. A product feature is one branch. The name says the outcome in plain words.

**Git's branch kinds, and no others:** `feature` for every initiative, product or tooling;
`bugfix` for a fix to `main`; `hotfix` for a fix to a release; `docs` for a documentation-only
change, which will be rare. `tooling` and `fix` are retired. A product feature's
`feature/<name>` still mirrors `docs/features/<name>.md`; a `feature/` branch without a
feature file is tooling, and the spec lane skips it visibly rather than blocking on a file
that was never meant to exist.

**The caps stay and are sized for an initiative:** 25 commits in no pull request, 4,000
changed lines. They catch a branch that never lands, not a branch that does its whole job.

**Pull requests are opened when the work is ready**, not as drafts on the first commit. A
session works the one branch it was asked for and reports rather than reacting; that rule is
in `CLAUDE.md`, as prompting.

Everything else in ADR 0013 stands: a merge commit, never a squash; a green gate or a written
override; `main` moves only by pull request.

## Consequences

- Fewer, larger pull requests. The gate's per-lane comments and inline findings carry more
  per run; the lanes were built for that.
- The size and commit caps are now loose enough that they will rarely fire. That is
  intended: they are a backstop, not the shape of the work.
- `feature/` no longer implies a feature file, so the spec lane's evidence gate reads the
  file's presence as "does this branch have a spec", not as missing evidence.

## What would change this

A branch that lives longer than a week and needs `main` merged into it more than twice is a
branch that should have been two. If that becomes the pattern, the caps come back down.

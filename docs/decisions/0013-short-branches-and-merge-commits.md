# ADR 0013 — Short branches, one pull request per step, merge commits

Date: 2026-09-16 · Status: Accepted · Amends ADR 0006

## Context

Five commits on `main`. Twenty on `base-setup` with no pull request. A gate branch that
forked from it, merged it back once, and now conflicts with it on `docs/STATE.md`. A test
branch nobody will merge. Nothing had said how a branch is named, how long it lives, or how it
lands, so each session chose — and the one open pull request, #1, carries the whole foundation
plus a rework of the gate that reviews it.

ADR 0006 says the gate's exit code, "wired to branch protection, is what actually stops the
merge". It is wired to nothing. The repository is private on a plan that answers 403 to
branch protection and rulesets alike, so the gate has never been able to stop a merge.

PR #1 was red for a reason that was neither a code finding nor a flake. Its reviewer step
finished in one turn with no model usage: the stored OAuth token had a leading space and the
API answered 401, which the action hid behind "output hidden for security". And the action
restores `.claude/` from `origin/main` because a pull request head is untrusted, so the first
PR to carry the rubric could not be reviewed by it. `main`'s own gate read an
`ANTHROPIC_API_KEY` that will never exist — the review bills to the subscription (ADR 0012).

## Decision

**One branch per feature step**, or per one self-contained tooling or docs change. Named
`<kind>/<slug>`, kind one of `feature`, `fix`, `docs`, `tooling`; `feature/<name>` mirrors
`docs/features/<name>.md`. The branch lives as long as the step. `base-setup` and
`gate/subscription-billing` predate this and land as they are.

**Landed with a merge commit.** Not squashed: the commit messages are where the depth lives —
the feature journal says "commits hold the depth" and points at them. Not rebase-merged: that
rewrites every SHA, so the branch Fedir read locally and the history on `main` stop being the
same objects, and the gate's merge-base arithmetic gets a moving target. A merge commit also
gives `git log --first-parent main` one line per pull request.

**The gate binds by convention until it can bind by mechanism.** A red gate is not merged. The
one way past a blocker is the `gate-override` label and a `Gate override: <reason>` line in the
body, which the gate copies into its comment so the decision stays on the record. `/pr land`
refuses anything else. The day branch protection is available, `gate` becomes the one required
check and this paragraph is superseded.

**Every pull request is opened, checked and landed through `/pr`**
(`.claude/skills/pr/SKILL.md`) from `.github/PULL_REQUEST_TEMPLATE.md`. Branches are deleted
on merge.

**The landing order for what exists now.** PR #1 first: with a valid token, the reviewer's
output shown, and a prompt that falls back to the PR's own copy of the rubric, it was the first
pull request the reviewer could judge — it passed, with two warnings. Then `base-setup` with
`origin/main` merged in, so the same reviewer judges what PR #1 did not contain. No override.
`test/gate-live` is deleted, unmerged.

## Consequences

- More pull requests, each smaller. Each one costs a review — API tokens on `main`'s gate,
  subscription quota once PR #1 lands (its ADR 0012) — so the unit is a step, not a commit.
- A pull request that adds an ADR checks the number against every open pull request's head,
  not only `main`. 0006 collided between sessions; 0010 collided between branches.
- `main` stays unprotected. `git push origin main` still works; the convention is the only
  thing that says not to, and `/pr` is the only path that follows it.
- No override was needed. A review that did not run is not evidence the code is clean — the fix
  was to make it run, and the reviewer's output is shown so the next failure has a cause.

## What would change this

- Branch protection becoming available: turn it on, require `gate`, drop the convention clause.
- A second regular contributor: review becomes human as well as agentic, and the template
  gains a reviewer line.
- Review cost becoming the reason steps get batched: reconsider the unit, not the rule.

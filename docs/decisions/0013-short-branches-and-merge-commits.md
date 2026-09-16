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

PR #1 is red for a reason that is neither a code finding nor a flake. Its reviewer step
finished in one turn with no model usage and no cost, and the action that runs it restores
`.claude/` from `origin/main` because the pull request head is untrusted — and `main` has no
`idiom-review` skill. The gate correctly reported `gate/review-incomplete`. And `main`'s own
gate reads an `ANTHROPIC_API_KEY` that was never created. No first pull request can be reviewed.

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

**The landing order for what exists now.** `base-setup` first, as a pull request to `main`,
carrying a copy of the `idiom-review` skill. Its reviewer cannot run — no key — so it lands by
override, judged on build, tests, launch, lint and the doc checks. Then PR #1 with `origin/main`
merged in: the skill is on `main` now, so its reviewer is the first that can run. `test/gate-live`
is deleted, unmerged.

## Consequences

- More pull requests, each smaller. Each one costs a review — API tokens on `main`'s gate,
  subscription quota once PR #1 lands (its ADR 0012) — so the unit is a step, not a commit.
- A pull request that adds an ADR checks the number against every open pull request's head,
  not only `main`. 0006 collided between sessions; 0010 collided between branches.
- `main` stays unprotected. `git push origin main` still works; the convention is the only
  thing that says not to, and `/pr` is the only path that follows it.
- The first override is the foundation's, and the only one this order needs. A review that did
  not run is not evidence the code is clean; the reason is on the record, and the next PR is reviewed.

## What would change this

- Branch protection becoming available: turn it on, require `gate`, drop the convention clause.
- A second regular contributor: review becomes human as well as agentic, and the template
  gains a reviewer line.
- Review cost becoming the reason steps get batched: reconsider the unit, not the rule.

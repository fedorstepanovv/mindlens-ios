# ADR 0017 — Review levels: the gate says how much a human reads, and a human always merges

Date: 2026-09-21 · Status: Accepted · Amends ADR 0013, 0015

## Context

ADR 0013 made every landing a human decision through `/pr land`, and ADR 0015 sized branches
for an initiative rather than a step. Neither said how much of a pull request the human reads
before merging, so the answer has been "all of it" — Fedir reads every diff. This repository
has one reviewer and, once the load starts, every pull request will be session-written. Reading
every diff in full is the bottleneck the pipeline exists to remove, and reading none of them
is how an unreviewed change reaches `main`.

What teams document splits in two. Stripe ("human-reviewed, but containing no human-written
code"), Cognition, Ramp and GitHub's Copilot docs review every agent pull request in full.
Anthropic tiers its codebases by risk and has Claude merge into the low-risk ones with a
"risk-weighted sample" read afterwards. Ona sits between: an agent reviews every pull request
and approves the ones meeting six objective criteria — under 1,000 lines, no migrations, no
infrastructure, no auth, no audit logging — "but a person always clicks the merge button", and
median lead time fell from 4.1 hours to 1.1.

## Decision

**A human always merges. The gate assigns a review level that says how much to read first.**

| Level | Human reads | Assigned when |
|---|---|---|
| **pass** | nothing — merge on the gate's approval | ≤200 changed lines **and** no risk path **and** no new ADR, dependency, or entitlement |
| **brief** | the gate's comment and the pull request body | everything not pass and not full |
| **full** | the diff | any risk path touched, or any lane finding at severity ≥ warning with a proof, or over 1,000 lines with an override |

The **risk class** forces full: `Packages/*/Sources/Networking/**`, `Packages/*/Sources/Persistence/**`,
`Config/**`, `.github/**`, `Tools/**`, `AGENTS.md`, `CLAUDE.md`, `.claude/**`, any file touching
Keychain or tokens (`Auth*`, `Keychain*`, `TokenRefresher*`), `*.entitlements`, `Info.plist`. The
list lives in `.github/swiftgate.yml`. The gate posts the level and its reasons in the scorer's
sticky comment and sets a `review:<level>` label; the pull request size cap is 1,000 changed
lines, Ona's published low-risk line, with `size-override` for the recorded exception.

The deterministic layer comes first, always: build, tests, lint, doc links and the static
blockers must be green before a level means anything. A level is advice about reading; the
green gate is the condition for merging.

## Alternatives rejected

- **Every pull request read in full** (Stripe 2026-02, Cognition 2026-02, Ramp 2026-01, GitHub
  Copilot docs). Correct with a team of reviewers; here it caps throughput at one person's
  reading speed and makes the gate's judgement redundant on the changes it was built for.
- **Agent-executed merges with post-merge sampling** (Anthropic, 2026-07-21). Needs a sample
  large enough to mean something, a protected `main`, and a rollback path. This repository has
  none of the three, and "the agent merged it" is not an answer this pipeline can give.
- **Two levels only**, pass and full. The middle one is where most pull requests will sit — the
  gate's comment plus the body is enough to catch a wrong direction without reading Swift.

## Consequences

- The mechanism — the criteria in `swiftgate.yml`, the label, the comment — lands with the
  gate branch that follows this one. Until it does, every pull request is read at full.
- `pass` will be rare at first: the risk class covers most of what setup work touches. That is
  intended; the level is earned by the objective criteria, not assigned to save time.
- The level is a label and a comment, not a lock. Nothing stops a human reading more; the rule
  only says what is enough.

## What would change this

A defect that landed through `pass` and would have been caught by reading the diff: the
criteria tighten, and the incident goes in `docs/LESSONS.md`. Or twenty product pull requests
with none at `pass`: the criteria are too tight to be useful, and the numbers in
`README.md` are the evidence for loosening them.

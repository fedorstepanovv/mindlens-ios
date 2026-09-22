# ADR 0021 — Lanes are advisory until they earn blocking on data, and a lane can be killed the same way

Date: 2026-09-21 · Status: Accepted · Amends ADR 0014, 0016

## Context

ADR 0014 called the lanes advisory and then made two of their four verdicts — `BLOCK` and
`CANNOT_EVALUATE` — stop the merge. ADR 0006's "what would change this" said a reviewer
overridden more than acted on should be demoted, and ADR 0016 built the records that could
say so; but the demotion was never the default, and the grant was never written down. Meanwhile
**no lane has judged a real diff**: every pull request since the three-lane gate landed
changed no Swift. The gate blocks on the word of judges whose noise rate is unknown.

Every vendor ships LLM review advisory by default and blocking by opt-in: Claude Code Review's
check "always completes with a neutral conclusion"; Bugbot defaults to `neutral` with an opt-in
`fail-on-unresolved-issues`; Codex review comments only on P0/P1; Copilot review can approve,
off by default and path-scoped (2026-09-01). Noise is controlled by proof, not thresholds:
Anthropic's substantive-comment rate went from 16% to 54% "by requiring the agents to write a
proof that their finding is valid" (2026-07-21); Shopify's verifier rejects "findings that cannot
be proven" (2026-07-29). No vendor publishes a precision number.

## Decision

The three lanes stay — narrow single-question judges, evidence-gated, scored deterministically.
Five rules change what their verdicts do:

1. **Advisory by default.** A lane's `BLOCK` or `CANNOT_EVALUATE` is reported and recorded, not
   a gate failure, until the lane has `blocks: true` in `.github/swiftgate.yml`.
   `CANNOT_EVALUATE` stays visible — a judge that could not look is never scored `PASS`.
2. **Grant rule.** `blocks: true` is set by a reviewed pull request that cites `swiftgate metrics`
   output: **≥10 judged pull requests, noise rate (`untouched ÷ classified`, ADR 0016) ≤30%,
   ≥1 finding classified `changed`.** Revoked the same way. No automation; the flip is the decision.
3. **Kill rule, pre-registered.** After **≥10 judged runs, noise >50% and zero findings
   classified `changed`**, the lane is deleted by a reviewed pull request that cites the numbers,
   with an ADR. Written before any record exists, so a cut is a decision on evidence we said we
   would use, not a reaction.
4. **Proof per finding.** The schema requires `proof`: the concrete input or state and the
   line where it fails, or the exemplar contradicted. A finding without one is not reported.
5. **Model per lane.** The runner's model is a parameter in `swiftgate.yml`, recorded in each
   metrics line, so a lane's noise rate is comparable across a model change. Advisory lanes need
   not run on Opus.

**The idiom lane is the one expected to fail the grant.** This repository's own Swift incidents
in `docs/LESSONS.md` are all correctness — concurrency, localization, test honesty — not
Flutter-isms; the mechanical Flutter-isms are caught by the static rules and SwiftLint; and
idiom is taste, the lowest-precision question of the three. It stays because it is the one lane
unique to the transfer story, and cutting it unrun forfeits the measured decision. Its cost is
bounded by a **narrow trigger** — it runs only when the diff *adds* a `View`, an `@Observable`
type, or a new file under a feature target, where structural translation happens — and by a
cheaper model.

The rename ADR 0012 still carries stands: `gate/review-incomplete` became `CANNOT_EVALUATE` in
ADR 0014, and this ADR keeps that verdict visible while removing its power to block.

## Alternatives rejected

- **Blocking from day one** — this repository, ADR 0014. Defensible when the alternative was a
  review that passed on an empty tree; indefensible for judges that have never run on a real diff.
- **A managed reviewer instead** — Claude Code Review ($15–25 a review, Team/Enterprise, research
  preview), Copilot review, Bugbot. Each is advisory, none is the transfer story, and none leaves
  a record this repository can measure. There would be nothing built and nothing to decide on.
- **Cutting the idiom lane now.** Forfeits the one measured decision the artifact is for.
- **A noise threshold with no proof requirement.** Thresholds count findings after the fact;
  proof reduces them before they are posted. Both, in that order.

## Consequences

- The mechanism — `blocks:`, `model:`, the trigger, `proof` in the schema — lands with the gate
  branch that follows this one. Until it does, `docs/STATE.md` says the lanes block.
- The first ten judged pull requests carry advisory verdicts a human reads at *brief* or *full*
  (ADR 0017). That is the load, and it is the cost of measuring before trusting.
- The grant and the kill are pull requests, so `git log` on `swiftgate.yml` is the history of
  which lane earned what and when.

## What would change this

The evidence bar in `README.md`: twenty product pull requests, every lane with a measured noise
rate, one lane granted, refused or killed on data. After that, this ADR's rules have either
worked or produced a number that says which rule was wrong.

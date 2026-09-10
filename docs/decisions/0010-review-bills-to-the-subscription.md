# ADR 0010 — The idiom review runs as Claude Code, billed to the subscription

Date: 2026-09-11 · Status: Accepted · Amends ADR 0006

## Context
ADR 0006 put a model in the merge path. It did so by calling the Messages API directly
from `Tools/swiftgate` with an `ANTHROPIC_API_KEY`, which bills per token to the Console
account — roughly $0.10–0.30 per pull request.

That is a second bill for a capability already paid for. A Claude subscription covers
Claude Code, and Claude Code can run in CI: `claude setup-token` mints a long-lived OAuth
token, and `anthropics/claude-code-action` accepts it as `CLAUDE_CODE_OAUTH_TOKEN`. Runs
authenticated that way draw on the subscription rather than API billing.

A subscription token cannot be swapped in behind the Go SDK. It authenticates Claude
Code, not arbitrary Messages API calls. Using it means the reviewer *is* Claude Code.

## Decision
The judgement half runs as `anthropics/claude-code-action@v1` inside the same job. The
gate splits in two around it:

```
swiftgate prepare   diff the PR, run the deterministic rules, write the brief
<Claude Code runs>  reads the brief, writes .swiftgate/run/agent-findings.json
swiftgate decide    merge both halves, comment on the PR, set the exit code
```

The decision stays in Go. The reviewer only produces findings; it does not decide, does
not comment, and has no path to the exit code. Its tools are `Read`, `Grep`, `Glob` and
`Write` — no Bash, no network.

The rubric moves from a Go string literal to `.claude/skills/idiom-review/SKILL.md`,
where Claude Code loads it as a skill and picks up `CLAUDE.md` for free.

The deterministic catalogue is unchanged. It never cost anything and it still doesn't.

## Consequences
The per-PR API charge goes away. The dependency tree drops to a single Go module, and
about 350 lines of SDK plumbing — a hand-rolled tool sandbox, a manual token accounting
loop — is deleted in favour of a harness that already does it.

Costs, honestly:

- **CI now draws on the same quota as interactive work.** The token is tied to one
  person's subscription. A busy day of migration plus a gate firing on every push can run
  into rate limits, and the failure lands on a pull request rather than in a terminal.
  This is the real trade and it is why the incomplete-review path below exists.
- **Token counts are no longer visible to the gate**, so the report can no longer print
  what a review cost. Spend moves to the subscription's own usage view.
- **Two processes instead of one**, coupled by files in `.swiftgate/run/`. The paths are a
  contract between `internal/review` and `SKILL.md`; they change together or not at all.
- **An OAuth token does not suit an organisation.** It belongs to whoever ran
  `setup-token`. If more than one person needs the gate, or it spreads to other
  repositories, go back to an API key — the deterministic half is unaffected either way.

**A review that did not run is not evidence that the code is clean.** A missing or
unparseable findings file becomes a blocking `gate/review-incomplete` finding that names
the likely cause and says to re-run. The alternative — treating a rate-limited run as a
pass — would make the gate lie exactly when it is under load.

## What would change this
Rate limits biting often enough that people re-run the gate to get past it, or a second
person needing to merge. Either one means moving the judgement half back to an API key.
The switch is one workflow step and one secret; nothing else in the gate depends on it.

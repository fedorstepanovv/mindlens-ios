# ADR 0016 — Every lane run leaves a record, and every merge says what became of each finding

Date: 2026-09-19 · Status: Accepted · Extends ADR 0014

## Context

ADR 0014 made the verdicts structured and named the question the next branch had to answer:
which lane earns its keep. Four merged pull requests had produced five findings and one
opinion about them — that the model's two strongest were verification auditing, not the
Flutter-ism detection the job was named for. That opinion was formed by reading the comments.
Nothing recorded what each lane said, what it cost, or whether anyone acted on it, so "which
lane earns its keep", "which rules never fire" and "what does a pull request cost" could only
be answered by re-reading every pull request. No lane has judged a real diff since the lanes
were split, so there are no real records to design against; there are fixtures with known
answers, and there is the one workflow output that carries cost and duration — the
`claude-code-action` execution file — whose real shape nobody here has seen.

## Decision

**One record per lane per run, written by `swiftgate decide`, uploaded as an artifact,
and never read by the scorer.** `Tools/swiftgate/internal/metrics` defines the record:
verdict, the cause when it is `CANNOT_EVALUATE` (`evidence`, `judge` or `output` — which of
ADR 0014's three harness conditions fired), the evidence result, the findings by rule, file,
line and severity, and, when the execution file says, cost, duration and turns. The static
pass is recorded as a fourth lane named `static`, so its rules are counted beside the judges'.
JSONL, one line each, `swiftgate-metrics-pr<N>-<run>-<attempt>`, kept 90 days.

**Cost and duration are optional and say so.** The record's doc comment names the file they
come from and that its shape is a guess: a JSON array of Claude Code stream messages ending
in a `result`, a single object, or one message per line are all accepted; anything else is
logged, and the field is absent — never zero, never a number about nothing. The raw file is
uploaded beside the records so the first real run settles the guess.

**At merge, each finding gets one of four outcomes**, read off git and the records by
`swiftgate outcomes --pr N`, in this precedence:

| Outcome | Read off | Counts as |
|---|---|---|
| `waived` | a `// swiftgate:allow <rule>` in the file at the head, or the override label | read and answered |
| `changed` | a hunk between the finding's SHA and the head covers its line, or the file is gone | acted on |
| `resolved` | the run at the head no longer reports it, and its lines did not change | acted on elsewhere, or a judge that was not consistent — counted apart so that shows |
| `untouched` | its lines are as they were, and it still stood or nothing ran at the head | noise |

The same finding on two pushes is one finding, dated from the first push. **Noise is
`untouched ÷ classified`**, per lane and per rule; a lane with nothing classified is
"unmeasured", not clean. `swiftgate metrics` sums both kinds of record into one report.

## Consequences

- The scorer is untouched. Records describe the gate; nothing in them can block a merge,
  and `decide` writes them before it decides so a blocked run is measured too.
- The question ADR 0014 left open — is a lane's `CANNOT_EVALUATE` firing on infrastructure
  or on evidence — is now a column, not an investigation.
- `resolved` will be inflated by judges that do not repeat themselves. That is a fact about
  the judges worth seeing, which is why it is not folded into `changed`.
- Everything here was verified against fixtures. The first real run may show the execution
  file is none of the three shapes, in which case cost reads "not reported" and the fix is
  one reader function, not the record.

## What would change this

A lane whose noise sits above half after twenty classified findings is a lane whose rubric
is wrong or whose findings nobody reads; either way it stops scoring until Phase 3's replay
says which. A rule that has never fired after fifty runs is a candidate to delete.

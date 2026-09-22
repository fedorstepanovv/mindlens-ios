# ADR 0022 — Memory: one small always-on file that routes, documents on demand, lessons kept apart with their guards

Date: 2026-09-21 · Status: Accepted · Extends ADR 0011

## Context

The instruction file reached its 150-line cap. Around it grew a set of files each doing one
job — `docs/STATE.md` rewritten in place at 80 lines, `docs/LESSONS.md` at 40 with a guard per
row, one file per feature (ADR 0011), the reference set under `docs/`, append-only ADRs and a
changelog — and the instruction file's main content became the table that says which to open.
The structure was never written down as a decision, so a session could reasonably add a
lesson to the instruction file, or a docs index, or a fourth kind of status file. Two did.

What vendors document converges. Claude Code: "target under 200 lines per CLAUDE.md file.
Longer files consume more context and reduce adherence"; move a procedure to a skill and a
one-area fact to its own file. Codex caps concatenated `AGENTS.md` at 32 KiB; Cursor rules at
500 lines; Copilot at "2 pages". Every one describes progressive disclosure — a tiny always-on
file, the rest on demand. On lessons, the documented habit is the opposite of this
repository's: "Anytime we see Claude do something incorrectly we add it to the CLAUDE.md"
(Cherny, 2026-01-02); Anthropic's memory guidance says add when "Claude makes the same
mistake a second time". Vercel's evals (2026-01-27) found an 8 KB docs index inside
`AGENTS.md` beat skills, 100% to 79%, with the skill "never invoked" in 56% of cases.

## Decision

- **`AGENTS.md` is the always-on file, ≤200 lines**, enforced by `Tools/check-doc-links.py`.
  It holds what every session needs in every task — the routing table, the one rule, the
  architecture and testing absolutes, how work lands — and nothing that only one task needs.
  `CLAUDE.md` is `@AGENTS.md` plus Claude-only lines, capped at 40.
- **Everything else is read on demand**, by the routing table: `docs/STATE.md` and
  `docs/LESSONS.md` always, then the one reference file the task touches. A session that
  reads the whole `docs/` tree has spent the context the task needed.
- **`docs/LESSONS.md` stays a separate file with a guard column.** A row is finished when
  something mechanical fails on the mistake's recurrence; a row whose guard is automated is
  deleted, because the guard is the memory. Forty lines is what forces the deletion.
- **`docs/STATE.md` is rewritten, never appended.** History goes to `CHANGELOG.md` and ADRs.
- **An agent's private memory binds nobody.** Anything meant to hold for the next session,
  another agent or a person is written to `AGENTS.md` or `docs/`.

## Alternatives rejected

- **Lessons inside the instruction file** (Cherny; Anthropic memory docs). Every lesson would
  cost always-on context forever, and none would carry a guard — the column is what turns "do
  not do X" into a check that fails when X recurs, and what lets a row be deleted. Twenty-three
  rows in the instruction file would be the bloat the vendors warn about.
- **A docs index inside `AGENTS.md`** (Vercel, 2026-01-27). The strongest evidence in the
  research, and it argues for the routing table this file already is; a full index would
  double the file. Revisit if the skills go uninvoked — the measure is whether a session
  reaches the right document, and the pull requests show it.
- **One `docs/` file per concern with no cap.** How the server repository's status file
  reached 2,000 lines.

## Consequences

- Two files hold the instructions, joined by an import. `AGENTS.md` is edited; `CLAUDE.md` is
  not, unless the Claude-only surface changes.
- The cap forces a choice on every added line: what comes out. That is the review.
- A lesson without a guard is prose, and prose gets relearned. The discipline is judgment;
  the file says so in its own header.

## What would change this

Sessions reaching the wrong document, or none, on tasks the routing table covers — measured
by pull requests that break a rule a routed file states. Then the index Vercel measured goes
into `AGENTS.md`, and the cap moves to fit it.

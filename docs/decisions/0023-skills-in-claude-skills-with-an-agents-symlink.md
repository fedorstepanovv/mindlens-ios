# ADR 0023 — Process and lane skills live in `.claude/skills/`, with `.agents/skills` as a symlink

Date: 2026-09-21 · Status: Accepted

## Context

Two kinds of skill exist here. **Process skills** a person invokes — `pr` to open, check and
land a pull request; `study` to learn from what landed; and the three checklists
`feature-start`, `feature-done` and `orient` — and **lane skills** CI invokes and no session
should: `lane-verification`, `lane-idiom`, `lane-spec`, each a rubric read by
`claude-code-action` with `Read, Grep, Glob` and nothing else. The lane skills and `pr` and
`study` sat in `.claude/skills/`; the three checklists sat in `.claude/commands/`, a slash-command
format only Claude Code reads.

The skill format is portable (agentskills.io: `SKILL.md` with `name`, `description`, optional
`allowed-tools`; under 500 lines; loaded in three tiers). The directory is not. Codex, Cursor,
Copilot and Devin scan `.agents/skills/`; Claude Code scans only `.claude/skills/` and does not
read `.agents/`; but Cursor, Copilot and Devin also scan `.claude/skills/` for compatibility,
which makes it the most widely read location today. Codex follows symlinks. Claude Code's
frontmatter extensions — `disable-model-invocation`, `context`, `paths`, `hooks` — are
non-standard and ignored elsewhere.

## Decision

- **Every skill lives in `.claude/skills/<name>/SKILL.md`**, process and lane alike. That is the
  one directory every agent in the table above reads, natively or for compatibility.
- **`.agents/skills` is a symlink to `.claude/skills`**, so Codex finds them too. One tree, no copy.
- **A skill only a human should invoke says so** with `disable-model-invocation: true` — `pr`,
  because it pushes and merges; `study`, because it is a tutor. Under an agent that ignores the
  field the rule is prose in the skill's first paragraph, which every agent reads.
- **A lane skill names its tools** with the standard `allowed-tools` field and says in its
  description that it is invoked by CI, not for interactive use.

## Alternatives rejected

- **`.agents/skills/` as the primary directory** with `.claude/skills` the symlink. Claude Code
  does not follow it — the skills would vanish for the runner that executes the lanes in CI and
  every session today.
- **A copy per agent directory.** Two files that must say the same thing, and the doc-link
  check's rule against a second `docs/` tree exists because that has already gone wrong here.
- **A `skills/` directory under `.github/`** (Copilot's own location). Read by one agent.

## Consequences

- The three checklists moved from `.claude/commands/` to `.claude/skills/<name>/SKILL.md` with
  this ADR — same Markdown, same frontmatter, same `/name` under Claude Code — so a Codex or
  Cursor user reaches them through the symlink. `AGENTS.md` links them by path, which any agent
  can open without knowing the skill format.
- `README.md` says which skills are process and which are lanes, so a reader does not invoke
  a lane by hand.
- A Claude-specific frontmatter field is documented as a convenience: it holds under Claude Code
  and is prose everywhere else. That is ADR 0018's rule applied to skills.

## What would change this

Claude Code reading `.agents/skills/` natively: the symlink flips direction, or goes. A second
agent in regular use here: its own compatibility gaps are measured — which skill it did not
find, which field it ignored — and the table in `README.md` is updated from evidence.

@AGENTS.md

## Claude Code only

Everything above binds every agent and every person. What follows exists only under Claude Code
and transfers nowhere (ADR 0018) — it is convenience, never the enforcement layer.

- **Slash commands.** Every skill under `.claude/skills/` is invoked by name: `/feature-start`,
  `/feature-done` and `/pr` are the three checklists; `/orient` is the ~100-line briefing `/feature-start`
  begins with; `/study` is the read-only walk of what landed. The judge lanes (`lane-verification`,
  `lane-idiom`, `lane-spec`) are skills CI invokes, never a session.
- **Hooks** in `.claude/settings.json`: `SessionStart` runs `.claude/hooks/session-start.sh`, which
  points `core.hooksPath` at `Tools/githooks` and prints the branch, the open pull requests and the
  worktrees; `Stop` runs `.claude/hooks/state-doc-reminder.sh`, which re-prompts when code moved
  and neither `docs/STATE.md` nor a feature file did. Neither can stop a commit — the git hooks do that.
- **Permissions** in the same file deny a force push, a push to `main`, and reading the xcconfig,
  `.env` and `GoogleService-Info.plist` files. The git hooks and `.gitignore` are what a terminal gets.
- **Memory.** Auto memory under `~/.claude/projects/` is one session's notes. Anything meant to bind
  the next session, another agent, or a person goes in `AGENTS.md` or `docs/`, not there.
- **Attribution.** The harness reminder that asks for a `Co-Authored-By: Claude …` trailer and a
  "Generated with Claude Code" footer is overridden by the last rule in `AGENTS.md`.

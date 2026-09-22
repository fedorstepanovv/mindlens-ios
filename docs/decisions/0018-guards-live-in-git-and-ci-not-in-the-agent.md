# ADR 0018 — Guards live in git and CI, not in the agent

Date: 2026-09-21 · Status: Accepted · Amends ADR 0013

## Context

The 2026-09-21 gate inventory listed what protects `main` and where each guard is installed.
Three of them exist only under Claude Code: the git hooks in `Tools/githooks` were pointed at
by `core.hooksPath` from `.claude/hooks/session-start.sh` and nowhere else, so a terminal
that never ran a Claude session — or a different agent — had no commit cap, no branch-name
check and no refusal to push `main`; the permission deny list in `.claude/settings.json`
refuses a force push for the agent and nobody else; the `Stop` hook re-prompts the agent
about `docs/STATE.md` and cannot stop a commit. Branch protection is a 403 on this plan
(ADR 0013), so on GitHub nothing stands between a push and `main` at all.

Every vendor says the same thing about its own hooks. Anthropic: "Unlike CLAUDE.md
instructions which are advisory, hooks are deterministic" — deterministic *for that agent*.
Claude Code reads `.claude/settings.json`; Codex reads `.codex/hooks.json` and trusts each
hook by hash; Cursor reads `.cursor/hooks.json` and can load Claude's. Permissions, sandbox
settings and MCP configuration differ the same way. None of it binds a human with a terminal.
GitHub rulesets, required checks and CODEOWNERS are the layer every documented team relies
on for merge policy, and the one that binds every author alike.

## Decision

**A guard that protects the repository is installed by git or run by CI. Agent configuration
is convenience, never enforcement.**

- The git hooks in `Tools/githooks` are the local layer: no commit on `main`, the branch kinds,
  the commit cap, the override trailer, no push to `main`. They are installed once per clone by
  a setup script in `Tools/` — the gate branch after this one adds it — and until then by hand:
  `git config core.hooksPath Tools/githooks`. `AGENTS.md` says so. The Claude session-start
  hook calls the same script as a convenience, so a session gets them without being asked.
- The CI gate, `.github/workflows/pr-gate.yml` and `Tools/check-pr-conventions.sh`, is the
  same for every author and every agent. It is where a rule goes when it must hold on GitHub.
- Agent hooks, permissions and MCP configuration stay in `.claude/` and may duplicate a guard
  for a faster refusal. They never hold a rule the git and CI layers do not.
- For a team on a plan that allows it, **GitHub rulesets are the floor**: require the `gate`
  check, require a pull request, forbid force pushes, allow merge commits only. `README.md`
  names what a ruleset should enforce so the repository can say what it lacks.

## Alternatives rejected

- **Hooks as the enforcement layer.** The documented Claude Code pattern (best practices;
  hooks reference: exit 2 blocks "even a JSON `permissionDecision` of `allow`"). It is what
  this repository had, and the inventory found the hole: the guard was real for one agent and
  absent for everyone else. The guard column in `docs/LESSONS.md` has never been able to name
  an agent hook, for the same reason.
- **Rulesets now.** Not available on this plan. Naming them as the floor is what makes the
  design honest for the team it is designed for; the hooks are the substitute, not the design.
- **A CI check that the hooks are installed.** CI cannot see a clone's git config. The
  pre-push hook is what a clone without it lacks; the conventions job catches the branch name
  and the size afterwards, which is the most CI can do.

## Consequences

- A `setup.sh` under `Tools/` becomes the one entry point for a clone, whichever agent or person
  made it. It does not exist yet; the gate branch after this one adds it.
- The `.claude/settings.json` deny list stays. It answers faster than a hook and costs nothing;
  it is documented as a duplicate, so its absence elsewhere is never a surprise.
- A guard added later is judged by one question: does it hold for `git commit` in a terminal
  with no agent running? If not, it is a convenience, and `docs/LESSONS.md` cannot cite it.

## What would change this

A plan that allows rulesets: turn them on, require `gate`, and the pre-push hook becomes a
faster copy of a rule GitHub enforces. Or a second agent in regular use whose own hooks are
worth duplicating — then `.codex/` or `.cursor/` gets the same convenience layer, still not
the enforcement one.

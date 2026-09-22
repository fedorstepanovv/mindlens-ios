#!/bin/bash
# One-time setup for a clone of this repository. Idempotent — run it as often as you like.
#
# Git hooks are not cloned, so a fresh checkout has none of the guards that protect main
# until something installs them. That something must not be an agent: hooks, permissions
# and MCP configuration bind one agent and transfer nowhere (ADR 0018), and the guards
# have to bind a terminal too. So this script is the install, `AGENTS.md` tells a human
# to run it, and .claude/hooks/session-start.sh calls it as a convenience for one agent
# that happens to open the repository first.
#
#   Tools/setup.sh
#
# It prints what it did and what it found. It changes nothing outside this clone.
set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

git rev-parse --git-dir >/dev/null 2>&1 || { echo "setup: not a git repository."; exit 1; }

status=0
note() { printf '  %s\n' "$*"; }

echo "Mindlens setup"

# --- git hooks ---------------------------------------------------------------------------------
# pre-commit refuses a commit on main, a mis-named branch, a branch that outgrows its pull
# request; pre-push refuses main and any non-fast-forward. Branch protection is a 403 on this
# plan, so these are the mechanism (ADR 0013).
if [ "$(git config --get core.hooksPath || true)" = "Tools/githooks" ]; then
  note "git hooks: already installed (core.hooksPath → Tools/githooks)"
else
  if git config core.hooksPath Tools/githooks; then
    note "git hooks: installed — core.hooksPath → Tools/githooks (pre-commit, pre-push)"
  else
    note "git hooks: COULD NOT INSTALL. Run: git config core.hooksPath Tools/githooks"
    status=1
  fi
fi
for hook in pre-commit pre-push; do
  [ -x "Tools/githooks/$hook" ] || { note "git hooks: Tools/githooks/$hook is not executable — chmod +x it"; status=1; }
done

# --- gh ----------------------------------------------------------------------------------------
# The conventions check reads the pull request's labels and body, the pre-commit cap reads the
# open-PR list, and /pr opens and lands. None of it works unauthenticated.
if ! command -v gh >/dev/null 2>&1; then
  note "gh: not installed. The PR conventions check and /pr need it — https://cli.github.com"
  status=1
elif gh auth status >/dev/null 2>&1; then
  note "gh: authenticated as $(gh api user --jq .login 2>/dev/null || echo '?')"
else
  note "gh: installed but not authenticated. Run: gh auth login"
  status=1
fi

# --- the gate ------------------------------------------------------------------------------------
# Not required to work here — CI builds it — but a local `go test ./...` is the fastest way to
# know a change to Tools/swiftgate is sound before pushing it.
if command -v go >/dev/null 2>&1; then
  note "go: $(go version | awk '{print $3}') — Tools/swiftgate builds locally"
else
  note "go: not installed. Only needed to work on Tools/swiftgate; CI builds it either way."
fi

# --- what a build still needs --------------------------------------------------------------------
# Gitignored and not ours to install: without it the app falls back to UnavailableIdentityProvider,
# and a Release build fails at launch (ADR 0010).
if [ -f mindlens/GoogleService-Info.plist ]; then
  note "firebase: mindlens/GoogleService-Info.plist present"
else
  note "firebase: mindlens/GoogleService-Info.plist missing — sign-in falls back to UnavailableIdentityProvider (ADR 0010)"
fi

if [ "$status" -eq 0 ]; then
  echo "Ready."
else
  echo "Setup incomplete — see the lines above."
fi
exit $status

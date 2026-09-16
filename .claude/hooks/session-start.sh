#!/bin/bash
# SessionStart hook: make the branch rules mechanical, then say where this checkout stands.
#
# 1. core.hooksPath → Tools/githooks. Git hooks are not cloned, so the first session to open
#    the repo installs them — and from then on they bind a terminal too, which CLAUDE.md alone
#    cannot do. Idempotent.
# 2. Refresh the open-PR cache the pre-commit hook reads, so a commit never waits on the network.
# 3. Print the orientation a session used to be asked to gather by hand: branch, distance from
#    main, whether this branch is in a pull request or inside someone else's, worktrees, and
#    branches going stale. Every line here is a question that, unasked, cost this repo a merge.
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 0
git rev-parse --git-dir >/dev/null 2>&1 || exit 0

common=$(git rev-parse --git-common-dir)
cache="$common/mindlens-open-prs"

# --- 1. hooks ---------------------------------------------------------------------------------
if [ "$(git config --get core.hooksPath || true)" != "Tools/githooks" ]; then
  git config core.hooksPath Tools/githooks && echo "Installed git hooks: core.hooksPath → Tools/githooks (pre-commit, pre-push)."
fi

# --- 2. fetch + cache -------------------------------------------------------------------------
git fetch --quiet --prune origin 2>/dev/null || echo "Could not fetch origin — the numbers below may be stale."
if command -v gh >/dev/null 2>&1; then
  if prs=$(gh pr list --state open --json number,headRefName --jq '.[] | "\(.number)\t\(.headRefName)"' 2>/dev/null); then
    printf '%s\n' "$prs" > "$cache"
  fi
fi

# --- 3. orientation ---------------------------------------------------------------------------
branch=$(git branch --show-current)
main=refs/remotes/origin/main
count() { git rev-list --count "$@" 2>/dev/null || echo "?"; }

echo "Branch: ${branch:-(detached)} — $(count "$main..HEAD") ahead of origin/main, $(count "HEAD..$main") behind."
if [ -n "$branch" ] && [ "$branch" != main ]; then
  if [ -f "$cache" ] && pr=$(awk -F'\t' -v b="$branch" '$2==b {print $1; exit}' "$cache") && [ -n "$pr" ]; then
    echo "Pull request: #$pr is this branch."
  else
    echo "Pull request: none for this branch. The pre-commit cap is ${MINDLENS_MAX_UNREVIEWED_COMMITS:-5} unreviewed commits; /pr opens one."
  fi
  if [ -f "$cache" ] && git rev-parse -q --verify "refs/remotes/origin/$branch" >/dev/null; then
    while IFS=$'\t' read -r num head; do
      [ -z "${head:-}" ] || [ "$head" = "$branch" ] && continue
      git rev-parse -q --verify "refs/remotes/origin/$head" >/dev/null || continue
      git merge-base --is-ancestor "refs/remotes/origin/$branch" "refs/remotes/origin/$head" \
        && echo "WARNING: origin/$branch is inside PR #$num ($head). Commits here re-diverge it — land #$num first."
    done < "$cache"
  fi
fi

if [ -f "$cache" ] && [ -s "$cache" ]; then
  echo "Open pull requests:"
  while IFS=$'\t' read -r num head; do
    [ -z "${head:-}" ] && continue
    ahead=$(count "$main..refs/remotes/origin/$head")
    echo "  #$num $head — $ahead commits not on main"
  done < "$cache"
fi

wt=$(git worktree list --porcelain | awk '/^worktree /{p=$2} /^branch /{print "  " p " [" substr($2,12) "]"}' | grep -v "^  $(git rev-parse --show-toplevel) " || true)
[ -n "$wt" ] && printf 'Worktrees (remove when their PR lands):\n%s\n' "$wt"

stale=$(git for-each-ref --format='%(refname:short) %(committerdate:unix)' refs/remotes/origin \
  | awk -v now="$(date +%s)" '$1!="origin/main" && $1!="origin/HEAD" && now-$2 > 7*86400 {sub("origin/","",$1); print $1}')
if [ -n "$stale" ] && [ -f "$cache" ]; then
  for b in $stale; do
    grep -q "	$b\$" "$cache" || echo "Stale: origin/$b — no open PR, no commit in 7 days. Land it or delete it."
  done
fi
exit 0

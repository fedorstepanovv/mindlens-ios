#!/bin/bash
# SessionStart hook: make the branch rules mechanical, then say where this checkout stands.
#
# 1. Tools/setup.sh — git hooks, gh, and what a build still needs. The script is the install and
#    a human runs it once per clone (AGENTS.md); calling it here only means the first session to
#    open the repo does not have to. The guard lives in the script, not in this hook (ADR 0018).
# 2. Refresh the open-PR cache the pre-commit hook reads, so a commit never waits on the network.
# 3. Print the orientation a session used to be asked to gather by hand: branch, distance from
#    main, whether this branch is in a pull request or inside someone else's, worktrees, and
#    branches going stale. Every line here is a question that, unasked, cost this repo a merge.
# 4. Warn when the main checkout is not on main. It once sat on a merged feature branch, 18
#    commits behind, and every gate number read from it was stale (AGENTS.md: one worktree per
#    branch; the main checkout stays on main).
set -uo pipefail
cd "$(dirname "$0")/../.." || exit 0
git rev-parse --git-dir >/dev/null 2>&1 || exit 0

common=$(git rev-parse --git-common-dir)
cache="$common/mindlens-open-prs"

# --- 1. setup ---------------------------------------------------------------------------------
# Quiet when there is nothing to do: its output is worth a session's attention only when
# something is missing.
if [ "$(git config --get core.hooksPath || true)" != "Tools/githooks" ] || ! Tools/setup.sh >/dev/null 2>&1; then
  Tools/setup.sh
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

# --- 4. the main checkout ---------------------------------------------------------------------
# The common dir is <main checkout>/.git wherever this runs, worktree or not.
maindir=$(dirname "$(git rev-parse --path-format=absolute --git-common-dir)")
mainbranch=$(git -C "$maindir" branch --show-current)
if [ "$mainbranch" != main ]; then
  echo "WARNING: the main checkout at $maindir is on '${mainbranch:-(detached)}', not main. Its numbers are stale — git -C $maindir switch main, and work in a worktree."
fi

echo "Branch: ${branch:-(detached)} — $(count "$main..HEAD") ahead of origin/main, $(count "HEAD..$main") behind."
if [ -n "$branch" ] && [ "$branch" != main ]; then
  if [ -f "$cache" ] && pr=$(awk -F'\t' -v b="$branch" '$2==b {print $1; exit}' "$cache") && [ -n "$pr" ]; then
    echo "Pull request: #$pr is this branch."
  else
    echo "Pull request: none for this branch. The pre-commit cap is ${MINDLENS_MAX_UNREVIEWED_COMMITS:-10} unreviewed commits; /pr opens one."
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

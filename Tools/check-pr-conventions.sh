#!/bin/bash
# Three checks the gate runs on every pull request, from ADR 0013. Each one is a mistake this
# repository has already made once.
#
#   branch   <kind>/<slug>, kind one of feature, bugfix, hotfix, docs (ADR 0015).
#   adr      a decision number this PR adds must not exist, under another name, on the base
#            branch or on the head of any other open pull request. 0006 collided between
#            sessions; 0010 collided between branches.
#   size     changed lines are capped at 1,000 — Ona's published low-risk line, adopted by
#            ADR 0017 — so "one pull request per initiative" stays one initiative and the
#            review level means something. Lock files, the project file and captured fixtures
#            do not count; swiftgate counts the same exclusions, so the cap the gate enforces
#            and the number the review level cites are one number. The `size-override` label
#            plus a body line `Size override: <why>` lifts the cap, and the reason is printed
#            into the log — the same shape as the gate's own override.
#
# Usage: Tools/check-pr-conventions.sh <pr-number>      (needs gh, and origin/main fetched)
set -uo pipefail

pr=${1:?pull request number}
cap=${MINDLENS_PR_SIZE_CAP:-1000}
repo=$(gh repo view --json nameWithOwner --jq .nameWithOwner)
base=refs/remotes/origin/main
failed=0
fail() { echo "::error::$*"; failed=1; }

read -r head labels body < <(gh pr view "$pr" --json headRefName,labels,body \
  --jq '[.headRefName, ([.labels[].name] | join(",")), (.body | @base64)] | @tsv')
body=$(printf '%s' "$body" | base64 --decode)

# --- branch ------------------------------------------------------------------------------------
case "$head" in
  feature/*|bugfix/*|hotfix/*|docs/*) echo "branch: $head" ;;
  *) fail "branch '$head' is not <kind>/<slug> with kind feature, bugfix, hotfix or docs (ADR 0015)" ;;
esac

# --- adr numbers -------------------------------------------------------------------------------
added=$(git diff --name-only --diff-filter=A "$base...HEAD" -- docs/decisions/ | grep -E '^docs/decisions/[0-9]{4}-' || true)
if [ -n "$added" ]; then
  others=$(gh pr list --state open --json number,headRefName --jq ".[] | select(.number != $pr) | \"\(.number)\t\(.headRefName)\"")
  for path in $added; do
    name=$(basename "$path"); num=${name:0:4}
    clash=$(git ls-tree --name-only "$base" docs/decisions/ | grep -E "^docs/decisions/$num-" | grep -vx "$path" || true)
    [ -n "$clash" ] && fail "ADR $num: this PR adds $name but main already has $(basename "$clash")"
    while IFS=$'\t' read -r onum ohead; do
      [ -z "${ohead:-}" ] && continue
      theirs=$(gh api "repos/$repo/contents/docs/decisions?ref=$ohead" --jq '.[].name' 2>/dev/null | grep -E "^$num-" | grep -vx "$name" || true)
      [ -n "$theirs" ] && fail "ADR $num: this PR adds $name but PR #$onum ($ohead) has $theirs — renumber before either lands"
    done <<< "$others"
    echo "adr: $name checked against main and every open pull request"
  done
else
  echo "adr: none added"
fi

# --- size --------------------------------------------------------------------------------------
changed=$(git diff --numstat "$base...HEAD" -- . ':(exclude)*.resolved' ':(exclude)*.pbxproj' ':(exclude)*/Fixtures/*' \
  | awk '$1!="-" {n+=$1+$2} END {print n+0}')
if [ "$changed" -gt "$cap" ]; then
  reason=$(printf '%s\n' "$body" | grep -iE '^\s*size[- ]override\s*:\s*\S' | head -1 | sed -E 's/^\s*[Ss]ize[- ][Oo]verride\s*:\s*//')
  if [[ ",$labels," == *",size-override,"* ]] && [ -n "$reason" ]; then
    echo "size: $changed changed lines, over the cap of $cap — overridden: $reason"
  elif [[ ",$labels," == *",size-override,"* ]]; then
    fail "size: $changed changed lines over the cap of $cap; the size-override label is set but the body has no 'Size override: <why>' line"
  else
    fail "size: $changed changed lines, cap is $cap (ADR 0015: one pull request per initiative). Split it, or add the size-override label and a 'Size override: <why>' line to the body."
  fi
else
  echo "size: $changed changed lines (cap $cap)"
fi

exit $failed

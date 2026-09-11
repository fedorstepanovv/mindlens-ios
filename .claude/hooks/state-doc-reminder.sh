#!/bin/bash
# Stop hook: if something that changes what is true changed, but docs/STATE.md didn't,
# say so.
#
# docs/STATE.md is the one file that tells a future session what is true. It rots the
# moment updating it depends on remembering to. This makes forgetting visible.
#
# Swift is not the only thing that moves the truth. The build settings, the project file,
# the CI gate and the check scripts all state what is true about this repo — and a change
# to Config/Base.xcconfig is exactly what once left every document claiming Swift 6 while
# the app built at Swift 5.

set -uo pipefail
payload=$(cat)

# Never fire twice in a row — the hook must not be able to trap the session in a loop.
if printf '%s' "$payload" | grep -q '"stop_hook_active"[[:space:]]*:[[:space:]]*true'; then
  exit 0
fi

cd "$(dirname "$0")/../.." || exit 0
git rev-parse --git-dir >/dev/null 2>&1 || exit 0

# -uall lists the files inside an untracked directory, not the directory. Without it a
# feature file created this session shows as `docs/features/` and never matches.
changed=$(git status --porcelain -uall 2>/dev/null)
# Strip the two status columns and their separator, leaving the path.
paths=$(printf '%s\n' "$changed" | sed 's/^...//')

# .claude/ is process — hooks and slash commands. Editing how a session is nagged does
# not change what is true about the project, and nagging about it is the noise that gets
# a guard switched off. Tools/ is different: a check script asserts project truth.
truth_changed=$(printf '%s\n' "$paths" \
  | grep -vE '^\.claude/' \
  | grep -E '\.(swift|xcconfig|pbxproj|py|sh)$|^\.github/workflows/.*\.ya?ml$' || true)
# Either file counts. docs/STATE.md is the ledger across features; docs/features/<name>.md
# is the one feature's steps and journal. A change to the code should move at least one.
state_changed=$(printf '%s\n' "$paths" | grep -E '^docs/(STATE\.md|features/[^/]+\.md)$' || true)

if [ -n "$truth_changed" ] && [ -z "$state_changed" ]; then
  echo "Code, build settings or a check changed but neither docs/STATE.md nor a docs/features/ file was updated." >&2
  echo "Tick the step and add a journal line in the feature file; update the ledger if the stage status moved. Or say why it doesn't apply." >&2
  exit 2
fi

exit 0

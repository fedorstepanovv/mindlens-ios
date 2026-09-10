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

changed=$(git status --porcelain 2>/dev/null)
# Strip the two status columns and their separator, leaving the path.
paths=$(printf '%s\n' "$changed" | sed 's/^...//')

# .claude/ is process — hooks and slash commands. Editing how a session is nagged does
# not change what is true about the project, and nagging about it is the noise that gets
# a guard switched off. Tools/ is different: a check script asserts project truth.
truth_changed=$(printf '%s\n' "$paths" \
  | grep -vE '^\.claude/' \
  | grep -E '\.(swift|xcconfig|pbxproj|py|sh)$|^\.github/workflows/.*\.ya?ml$' || true)
state_changed=$(printf '%s\n' "$paths" | grep -E '^docs/STATE\.md$' || true)

if [ -n "$truth_changed" ] && [ -z "$state_changed" ]; then
  echo "Code, build settings or a check changed but docs/STATE.md was not updated." >&2
  echo "Update the ledger (status, what works, what's deferred) before finishing, or say why it doesn't apply." >&2
  exit 2
fi

exit 0

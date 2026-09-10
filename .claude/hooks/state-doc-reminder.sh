#!/bin/bash
# Stop hook: if Swift changed but docs/STATE.md didn't, say so.
#
# docs/STATE.md is the one file that tells a future session what is true. It rots the
# moment updating it depends on remembering to. This makes forgetting visible.

set -uo pipefail
payload=$(cat)

# Never fire twice in a row — the hook must not be able to trap the session in a loop.
if printf '%s' "$payload" | grep -q '"stop_hook_active"[[:space:]]*:[[:space:]]*true'; then
  exit 0
fi

cd "$(dirname "$0")/../.." || exit 0
git rev-parse --git-dir >/dev/null 2>&1 || exit 0

changed=$(git status --porcelain 2>/dev/null)
swift_changed=$(printf '%s\n' "$changed" | grep -E '\.swift$' || true)
state_changed=$(printf '%s\n' "$changed" | grep -E 'docs/STATE\.md$' || true)

if [ -n "$swift_changed" ] && [ -z "$state_changed" ]; then
  echo "Swift files changed but docs/STATE.md was not updated." >&2
  echo "Update the ledger (status, what works, what's deferred) before finishing, or say why it doesn't apply." >&2
  exit 2
fi

exit 0

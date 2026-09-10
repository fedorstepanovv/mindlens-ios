#!/bin/bash
# Re-capture API fixtures from the live server.
#
# There is no OpenAPI spec, so these captures are the contract guard: when the server
# changes shape, the decoding tests fail. That only works if refreshing them is a
# command rather than a copy-paste chore — which is what this is.
#
#   Tools/capture-fixtures.sh              # unauthenticated fixtures only
#   TOKEN=<access-token> Tools/capture-fixtures.sh   # everything

set -euo pipefail

BASE="${API_BASE_URL:-https://mindlens-api-production.up.railway.app}"
DIR="$(cd "$(dirname "$0")/.." && pwd)/Packages/MindlensKit/Sources/TestSupport/Fixtures"

capture() {
  local name=$1 method=$2 path=$3 auth=$4
  shift 4
  local args=(-sS -m 20 -X "$method" "$BASE$path" -o "$DIR/$name.json" -w '%{http_code}')
  [ "$auth" = "auth" ] && args+=(-H "Authorization: Bearer ${TOKEN:-}")
  [ $# -gt 0 ] && args+=(-H 'Content-Type: application/json' -d "$1")

  local code
  code=$(curl "${args[@]}") || { echo "  ✗ $name — request failed"; return 1; }
  echo "  ✓ $name  (HTTP $code)"
}

echo "Capturing from $BASE"

echo "Unauthenticated:"
capture health_ready_200        GET  /health/ready ""
capture error_unauthorized_401  GET  /users        ""
capture error_validation_400    POST /auth/apple   "" '{}'
capture error_unknown_field_400 POST /auth/apple   "" \
  '{"idToken":"x","guid":"abcdef","deviceModel":"iPhone17","timezone":"Europe/Kyiv","surprise":1}'

if [ -n "${TOKEN:-}" ]; then
  echo "Authenticated:"
  capture user_200      GET /users              auth
  capture profile_200   GET /profiles           auth
  capture categories_200 GET /categories        auth
  capture mood_list_200 GET "/mood?date=$(date +%F)&timezone=Europe/Kyiv" auth
else
  echo "Authenticated: skipped (set TOKEN to capture)"
fi

echo
echo "Now run the tests — a shape change shows up as a decoding failure:"
echo "  cd Packages/MindlensKit && swift test"

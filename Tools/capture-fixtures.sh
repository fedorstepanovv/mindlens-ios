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

# Captures to a temp file and only replaces the fixture on success.
#
# `-o` truncates the target before curl knows how the request went, and `-sS` exits 0 for any
# HTTP status — so a 502 HTML page used to *become* the fixture, and a dropped connection left an
# empty one. These files are the only contract guard in the repo; one bad run should not be able
# to destroy them.
capture() {
  local name=$1 method=$2 path=$3 auth=$4 expect=$5
  shift 5
  local tmp
  tmp=$(mktemp)
  local args=(-sS -m 20 -X "$method" "$BASE$path" -o "$tmp" -w '%{http_code}')
  [ "$auth" = "auth" ] && args+=(-H "Authorization: Bearer ${TOKEN:-}")
  [ $# -gt 0 ] && args+=(-H 'Content-Type: application/json' -d "$1")

  local code
  if ! code=$(curl "${args[@]}"); then
    rm -f "$tmp"
    echo "  ✗ $name — request failed, kept the existing fixture"
    return 1
  fi

  if [ "$code" != "$expect" ]; then
    rm -f "$tmp"
    echo "  ✗ $name — HTTP $code, expected $expect. Kept the existing fixture."
    return 1
  fi

  if ! head -c 1 "$tmp" | grep -q '[[{]'; then
    rm -f "$tmp"
    echo "  ✗ $name — response was not JSON. Kept the existing fixture."
    return 1
  fi

  mv "$tmp" "$DIR/$name.json"
  echo "  ✓ $name  (HTTP $code)"
}

echo "Capturing from $BASE"

echo "Unauthenticated:"
capture health_ready_200        GET  /health/ready "" 200
capture error_unauthorized_401  GET  /users        "" 401
capture error_validation_400    POST /auth/apple   "" 400 '{}'
capture error_unknown_field_400 POST /auth/apple   "" 400 \
  '{"idToken":"x","guid":"abcdef","deviceModel":"iPhone17","timezone":"Europe/Kyiv","surprise":1}'
# A well-formed body with a bogus Firebase token: passes class-validator, fails
# verification. This is the only auth failure the sign-in UI has to explain to a user.
capture error_invalid_token_422  POST /auth/apple   "" 422 \
  '{"idToken":"not-a-real-firebase-token","guid":"11111111-2222-3333-4444-555555555555","deviceModel":"iPhone17,1","timezone":"Europe/Kyiv"}'

if [ -n "${TOKEN:-}" ]; then
  echo "Authenticated:"
  capture user_200      GET /users              auth 200
  capture profile_200   GET /profiles           auth 200
  capture categories_200 GET /categories        auth 200
  capture mood_list_200 GET "/mood?date=$(date +%F)&timezone=Europe/Kyiv" auth 200
  capture mood_create_200 POST /mood auth 201 '{"moodRate":3}'
  echo "  … /auth/apple 200 and /auth/refresh 200 are never captured: the first needs a live"
  echo "    Firebase ID token, the second consumes the session's refresh token. They are"
  echo "    TestSupport/ConstructedResponse, cross-checked instead (ADR 0024)."
else
  echo "Authenticated: skipped (set TOKEN to capture)"
fi

echo
echo "Now run the tests — a shape change shows up as a decoding failure:"
echo "  cd Packages/MindlensKit && swift test"

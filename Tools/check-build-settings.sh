#!/usr/bin/env bash
# Assert that the build settings the documents claim are the settings that actually build.
#
# Config/Base.xcconfig once sat in this repo unreferenced by the Xcode project: every
# document said Swift 6 / iOS 18 while the target built at Swift 5 / iOS 26.2. The same
# file's API_BASE_URL was quietly losing its "//" to the xcconfig comment rule. Neither
# was visible until something resolved the settings, so this resolves them.
#
# Usage: Tools/check-build-settings.sh [destination]
set -euo pipefail

cd "$(dirname "$0")/.."

destination="${1:-generic/platform=iOS Simulator}"
settings=$(xcodebuild -showBuildSettings -scheme mindlens -configuration Debug \
    -destination "$destination" 2>/dev/null)

status=0
check() {
    local key=$1 want=$2 got
    got=$(printf '%s\n' "$settings" | sed -n "s/^ *$key = //p" | head -1)
    if [ "$got" != "$want" ]; then
        printf '  %-30s is %-26s expected %s\n' "$key" "'${got:-unset}'" "'$want'"
        status=1
    fi
}

check SWIFT_VERSION 6.0
check IPHONEOS_DEPLOYMENT_TARGET 18.0
check SWIFT_STRICT_CONCURRENCY complete
check API_BASE_URL https://mindlens-api-production.up.railway.app

if [ "$status" -ne 0 ]; then
    cat <<'MSG'

These are defined in Config/Base.xcconfig, which is the project's base configuration
(ADR 0009). A target-level entry in mindlens.xcodeproj/project.pbxproj silently wins
over it — that is the usual cause, and Xcode writes one whenever a value is edited
through the build-settings inspector.
MSG
    exit 1
fi

echo "Build settings OK — Swift 6, iOS 18, complete concurrency, API URL intact."

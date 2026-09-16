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

# API_BASE_URL only reaches runtime through the app's Info.plist, and the app reads it at
# launch — a missing key is a crash on the first screen, not a warning.
#
# This reads the **built** Info.plist rather than a build setting, because the two disagreed
# once already: INFOPLIST_KEY_MindlensAPIBaseURL resolved perfectly in -showBuildSettings and
# never reached the bundle, since Xcode forwards only INFOPLIST_KEY_ names on its own allowlist
# and silently drops the rest. A setting that resolves is not a setting that shipped.
want_url=https://mindlens-api-production.up.railway.app
products=$(printf '%s\n' "$settings" | sed -n 's/^ *BUILT_PRODUCTS_DIR = //p' | head -1)
plist_path=$(printf '%s\n' "$settings" | sed -n 's/^ *INFOPLIST_PATH = //p' | head -1)
plist="$products/$plist_path"

if [ ! -f "$plist" ]; then
    printf '  %-30s %s\n' "Info.plist" "not built yet — build the app for this destination first"
    status=1
elif ! got_url=$(plutil -extract MindlensAPIBaseURL raw "$plist" 2>/dev/null) \
    || [ "$got_url" != "$want_url" ]; then
    printf '  %-30s is %-26s expected %s\n' "MindlensAPIBaseURL" "'${got_url:-absent}'" "'$want_url'"
    printf '  %s\n' "(in $plist — it comes from mindlens/Info.plist, which \$(API_BASE_URL) fills in)"
    status=1
fi

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

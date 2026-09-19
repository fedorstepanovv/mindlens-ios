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
# Unset, Xcode fills the team in from whichever account is signed in — and it picked one that
# did not own the App ID, so Sign in with Apple failed with AKAuthenticationError -7022 on the
# first real run. The team is the one that holds com.trymindlensnative.mindlens.
check DEVELOPMENT_TEAM 8N7NUUCQ9X

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

# Sign in with Apple is an entitlement, and an entitlement is only real once it is in the built
# executable. Not the code signature: a simulator build signs ad hoc with an *empty* set, so
# `codesign -d --entitlements` reads back `{}` and looks exactly like the capability was dropped.
# The simulator enforces the __TEXT,__entitlements section instead, so that is what this reads.
# otool prints the section as little-endian 32-bit words; the awk swaps each back into bytes.
exe_path=$(printf '%s\n' "$settings" | sed -n 's/^ *EXECUTABLE_PATH = //p' | head -1)
exe="$products/$exe_path"
if [ ! -f "$exe" ]; then
    printf '  %-30s %s\n' "executable" "not built yet — build the app for this destination first"
    status=1
elif ! otool -X -s __TEXT __entitlements "$exe" \
    | awk '{for (i = 2; i <= NF; i++) printf "%s", substr($i,7,2) substr($i,5,2) substr($i,3,2) substr($i,1,2)}' \
    | xxd -r -p | tr -d '\0' \
    | plutil -extract 'com\.apple\.developer\.applesignin' json -o - - >/dev/null 2>&1; then
    printf '  %-30s %s\n' "applesignin entitlement" "absent from the built executable"
    printf '  %s\n' "(mindlens/mindlens.entitlements, via CODE_SIGN_ENTITLEMENTS on the app target)"
    status=1
fi

# Google Sign-In redirects to a custom URL scheme — the plist's REVERSED_CLIENT_ID — and the
# scheme has to be declared statically in Info.plist. The SDK asserts it at the tap with an
# Objective-C exception; the provider throws first; this reads both files off the built bundle
# so the mismatch is a failed check, not a failed run. Only when a plist is bundled: CI and a
# fresh clone have none, and cannot sign in with anything anyway. The value is never printed.
resources_path=$(printf '%s\n' "$settings" | sed -n 's/^ *UNLOCALIZED_RESOURCES_FOLDER_PATH = //p' | head -1)
google_plist="$products/$resources_path/GoogleService-Info.plist"
if [ -f "$google_plist" ]; then
    reversed=$(plutil -extract REVERSED_CLIENT_ID raw "$google_plist" 2>/dev/null || true)
    if [ -z "$reversed" ]; then
        printf '  %-30s %s\n' "REVERSED_CLIENT_ID" "absent from the bundled GoogleService-Info.plist"
        printf '  %s\n' "(enable Google as a sign-in provider in the Firebase project and re-download the plist)"
        status=1
    elif ! plutil -extract CFBundleURLTypes json -o - "$plist" 2>/dev/null | grep -qF "\"$reversed\""; then
        printf '  %-30s %s\n' "Google URL scheme" "the bundled plist's REVERSED_CLIENT_ID is not in CFBundleURLSchemes"
        printf '  %s\n' "(declare it in mindlens/Info.plist under CFBundleURLTypes; Google cannot redirect back without it)"
        status=1
    fi
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

google_note="no GoogleService-Info.plist bundled"
[ -f "$google_plist" ] && google_note="Google URL scheme declared"
echo "Build settings OK — Swift 6, iOS 18, complete concurrency, API URL intact, Sign in with Apple entitled, $google_note."

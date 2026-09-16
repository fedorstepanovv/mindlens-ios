# ADR 0009 — Build settings live in xcconfig, applied at the project level

Date: 2026-09-10 · Status: Accepted

## Context
`Config/Base.xcconfig` was written with the correct values — iOS 18, Swift 6, complete
concurrency checking — and then never referenced by the Xcode project. The project kept
its template defaults, so `SWIFT_VERSION = 5.0` and `IPHONEOS_DEPLOYMENT_TARGET = 26.2`
were what actually built while every document said otherwise. `docs/LESSONS.md` already
carried an entry about exactly this class of false claim; the file existing was mistaken
for the setting being applied.

Two further facts shaped the fix. A setting written into a target's `buildSettings`
**overrides** a project-level xcconfig, so leaving the template values in place would
have silently defeated the file. And the value `URL_SCHEME_SEPARATOR = //` did not do
what it looked like: xcconfig treats `//` as the start of a comment even on the right of
an `=`, so `API_BASE_URL` had been resolving to `https:mindlens-api-production…` with the
slashes eaten. Nothing caught it because nothing had ever resolved the setting.

## Decision
`Config/Base.xcconfig` is attached as the `baseConfigurationReference` of the **project's**
Debug and Release configurations, not the app target's. The corresponding
`SWIFT_VERSION` and `IPHONEOS_DEPLOYMENT_TARGET` entries were deleted from all six target
configurations so the xcconfig is the only definition.

The scheme separator is assembled from two `$(URL_SCHEME_SLASH)` expansions of a single
`/`, which the comment rule does not touch.

## Consequences

One place defines the language mode and deployment floor, and it is a diffable text file
rather than a 600-line plist — a reviewer can see a deployment-target change in a pull
request without reading `project.pbxproj`.

Applying it at the project level rather than the app target's means the test targets
inherit the same floor instead of drifting; `mindlensTests` had been pinned to 26.2
independently. Targets that genuinely need a different value must now override
deliberately, and that override is visible as an addition rather than a leftover.

The cost is indirection: a setting shown in Xcode's build-settings inspector now comes
from a file that must be opened separately, and Xcode will happily write a new
target-level override on top of it if someone edits a value through the UI. That is the
failure mode this ADR exists to warn about.

## What would change this
A target that legitimately needs its own deployment floor — a widget or App Intents
extension (ADR 0008) would be the first. At that point add a second xcconfig that
`#include`s the base rather than reintroducing settings into `project.pbxproj`.

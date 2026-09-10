# ADR 0005 — Third-party SDKs behind protocols, stubs by default

Date: 2026-09-10 · Status: Accepted

## Context
The Flutter app carries RevenueCat, OneSignal, Sentry, PostHog and AppsFlyer. Calling
them directly from feature code would mean tests need credentials, the app can't build
without five sets of keys, and vendor types leak into view models.

## Decision
Each capability gets a protocol we own — `AnalyticsRecording`, `PurchaseProviding`,
`PushRegistering`, `CrashReporting`. Stub implementations are wired by default. Real
SDKs are SPM dependencies of the **app target only**, bound at the composition root.

Feature targets never import a vendor SDK. Vendor types never appear in a signature.

## Consequences
The app builds and the full test suite runs with no credentials at all. Swapping a vendor
touches one file. Analytics assertions become ordinary unit tests against a recording
stub.

Cost: a thin mapping layer per SDK, and vendor-specific features need a deliberate
decision to expose rather than being available by default. That constraint is doing its
job.

Two integration details that must survive the abstraction, because both fail silently:
- OneSignal external ID must be exactly `mindlens-user-<userId>`.
- RevenueCat needs the AppsFlyer device ID set for server-to-server revenue attribution.

## What would change this
A vendor SDK whose value is inseparable from its own UI — RevenueCat's prebuilt paywalls
being the likely case. Then the app target uses it directly and features still don't.

# ADR 0002 — Local SPM package with per-feature targets

Date: 2026-09-10 · Status: Accepted

## Context
The Flutter app is feature-first with 18 vertical slices, but the boundaries are
convention only — any file can import any other. In practice that produced cross-feature
imports that made features impossible to reason about in isolation.

## Decision
One local package, `Packages/MindlensKit`, with a target per feature plus shared targets
(`Core`, `Models`, `Networking`, `Persistence`, `DesignSystem`, `Analytics`,
`TestSupport`). **A feature target may not depend on another feature target.**

## Consequences
The dependency rule is enforced by the compiler rather than by review, so it cannot
erode. Test targets are small and fast because they link only what they need. Cross-
feature navigation must go through typed destinations resolved by the app target — more
indirection, but it makes coupling visible instead of accidental.

Cost: adding a feature means editing `Package.swift`, and `Package.swift` becomes a file
worth reviewing carefully.

Chose a single multi-target package over multiple packages: same enforcement, far less
manifest churn.

## What would change this
Build times growing enough to warrant splitting into separate packages with binary
caching, or a feature genuinely needing to ship independently.

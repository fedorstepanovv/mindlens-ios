# ADR 0003 — SwiftData for local persistence, behind repository protocols

Date: 2026-09-10 · Status: Accepted

## Context
The Flutter app has no local database. It refetches everything on launch and keeps a
30-entry in-memory day cache, so a cold launch shows an empty screen and logging a mood
offline loses the entry. For a daily-logging app, both are real product defects.

Options considered: Core Data (mature, verbose), GRDB (excellent async/actor story,
third-party), SwiftData (Apple-current, greenfield-friendly), or staying stateless.

## Decision
SwiftData, as the source of truth the UI observes, with the network syncing into it and
a persisted outbox for writes on the core logging path (mood, tags, notes). All of it
behind repository protocols in feature code.

## Consequences
Cold launch paints instantly from disk and logging works offline — a visible quality
difference from the Flutter app.

The honest cost: SwiftData has real friction under Swift 6 strict concurrency.
`ModelContext` is not `Sendable`, which pushes ownership into `@ModelActor` types and
makes some patterns awkward. GRDB is cleaner there. At this data volume — one user's
daily records — the friction is manageable and the Apple-native choice is worth more.

Because persistence sits behind protocols, the engine is swappable without touching
feature code. That is the part that actually matters.

## What would change this
Migration pain on a schema change, concurrency friction that starts costing real time, or
a dataset large enough that query performance matters. Any of those → GRDB.

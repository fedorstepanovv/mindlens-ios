# ADR 0003 — SwiftData for local persistence, behind repository protocols

Date: 2026-09-10 · Status: Accepted (revised after architecture review)

## Context
The Flutter app has no local database. It refetches everything on launch and keeps a
30-entry in-memory day cache, so a cold launch shows an empty screen and logging a mood
offline loses the entry. For a daily-logging app, both are product defects.

Options: Core Data, GRDB, SwiftData, or staying stateless.

## Decision
SwiftData, as the source of truth the UI observes, with the network syncing into it and a
persisted outbox for writes on the core logging path. All behind repository protocols, so
feature code never names the engine.

## Consequences

Cold launch paints instantly and logging works offline.

**The costs, stated plainly** — the first version of this ADR said "the friction is
manageable," which was not specific enough to be useful:

- **`PersistentModel` is not `Sendable`.** Models cannot cross an actor boundary at all.
  You pass `PersistentIdentifier` and re-fetch on the other side. This is the dominant
  ergonomic cost.
- **`@ModelActor` reentrancy.** Any `await` inside a method releases the actor and lets
  another method mutate the same context mid-operation. The mitigation — "don't `await`
  inside a `@ModelActor` method except at boundaries" — is a convention the compiler
  cannot enforce, which is exactly the kind of rule ADR 0002 says erodes.
- **`#Predicate` cannot express this app's core query.** No arbitrary function calls, so
  no `Calendar.isDate(_:inSameDayAs:)`. "Moods for a local day" needs a **stored**,
  normalised day key, re-derived whenever the user's timezone changes — and
  `PATCH /users/timezone` exists, so it does change.
- **Migrations run synchronously at container init.** A failure is a launch crash with no
  rollback.

Honest assessment: this architecture — value-typed domain models, an observable source of
truth, a durable ordered outbox — is **GRDB-shaped**. `ValueObservation` would give us the
observable-store-returning-value-types property in one line, and full SQL makes the
local-day query trivial. We are choosing SwiftData anyway because it is Apple's current
framework and demonstrating it has value for this project's purpose. Naming that reason is
more honest than pretending the technical case is clean.

## What would change this

The previous version listed "migration pain, concurrency friction, dataset size" — every
one of which fires **after** users have data, when switching is hardest. That was a
deferral dressed as a decision.

Replaced with a **pre-ship gate**. Before 1.0, and while the store is still empty:

1. Execute a real `VersionedSchema` migration — add a field *and* a relationship — against
   a store seeded with ~5,000 records.
2. Implement the local-day query and the outbox drain under `@ModelActor`.

If either requires `@unchecked Sendable`, or takes materially longer than expected, switch
to GRDB **then**. After users have data, switching means writing a migration *out of*
SwiftData, which is strictly harder than whatever triggered it.

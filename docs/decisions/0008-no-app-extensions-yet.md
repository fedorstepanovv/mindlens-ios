# ADR 0008 — No app extensions in v1, and the Keychain decision that depends on it

Date: 2026-09-10 · Status: Accepted

## Context
An App Intent plus a Control Center control for one-tap mood logging is the single
highest-signal native feature this app could have: it is genuinely good product design for
a daily-logging app, and it is the thing Flutter structurally cannot do well.

It also requires an app extension, an App Group, and a **Keychain access group** so the
extension can read the auth token. Retrofitting a Keychain access group after release
means migrating every existing user's Keychain item.

## Decision
No app extensions in v1. Focus stays on the four feature stages in `docs/STATE.md`.

We are also **not** adding a Keychain access group now. Adding the entitlement today would
require signing configuration the project does not have (`DEVELOPMENT_TEAM` is unset) and
would break local builds for no present benefit.

## Consequences

Scope stays bounded, and the app ships without an extension target to maintain.

The migration risk people worry about does not currently exist: **the app has never
shipped, so there are no Keychain items to migrate.** The cost of this decision is zero
until first release, and only then becomes real.

`KeychainTokenStorage` already stores both tokens as a **single item**, which means adding
an access group later is a one-line attribute change plus a migration of one item rather
than several.

## What would change this
Preparing the first App Store release. At that point, decide the access group **before**
submitting — after that, changing it costs a user-data migration. Treat this as a
pre-ship gate, in the same shape as ADR 0003's.

# ADR 0004 — Token refresh as an actor with failure classification

Date: 2026-09-10 · Status: Accepted

## Context
The backend issues 15-minute access tokens and **single-use, rotating** refresh tokens:
`POST /auth/refresh` invalidates the old refresh token and returns a new pair.

That makes concurrent refresh a correctness bug, not an edge case. A dashboard load
firing several requests at once will 401 together; whichever refresh lands second is
using an already-consumed token and fails permanently.

This is not hypothetical. It happened in production on 2026-07-12 and force-logged-out
users. The Flutter fix — a `QueuedInterceptor` plus a `Completer` — works, but the
mechanism is incidental to the language rather than expressed by it.

## Decision
A `TokenRefresher` actor owns refresh. Concurrent callers join a single in-flight
`Task` rather than starting their own. Failures are classified:

- **400/401/403** → the refresh token is genuinely dead. Sign the user out.
- **Timeout, 5xx, decode failure** → transient. Keep the session and let the caller retry.

## Consequences
Single-flighting becomes structural — actor isolation makes the race unrepresentable,
rather than prevented by a queue that someone could later bypass.

The classification is the half that actually caused the incident: treating every failed
refresh as "log out" turns a thirty-second backend blip into mass sign-outs. It is
directly testable, and it is tested.

## What would change this
The server dropping refresh-token rotation, which would make concurrent refresh harmless.

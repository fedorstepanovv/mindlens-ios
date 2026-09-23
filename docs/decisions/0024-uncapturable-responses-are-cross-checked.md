# ADR 0024 — The two uncapturable auth responses are cross-checked, not captured

Date: 2026-09-22 · Status: Accepted · Narrows ADR 0007 (captured-fixture contract tests) for two responses

## Context

The rule in `AGENTS.md` and `docs/TESTING.md` is that every API response type is decoded against
a real captured fixture, since with no OpenAPI spec the fixtures are the only contract guard. Two
responses cannot be captured the usual way:

- `POST /auth/apple` (and `/auth/google`) needs a live Firebase ID token, which only the app holds.
- `POST /auth/refresh` consumes the session's single-use refresh token, so calling it beside a
  running app signs the app out.

`docs/features/auth.md` step 5 planned to capture both through a proxy watching the app's traffic.
On 2026-09-22 that failed: the simulator rejected mitmproxy's root certificate at the TLS handshake
even after `simctl keychain add-root-cert`, and every request ended with "Can't reach Mindlens".

Meanwhile the shape had three independent readings, and they agree on every key each of them reads:
1. The server's Prisma `User` model and `Tokens` type, which produce the response.
2. The source app's `UserModel` and `TokensModel` (`../app/mindlensapp`). These decode these exact
   responses in production today, and all eight `UserModel` keys are `required`. It does not read
   `timezone`, which our `User` requires. Prisma declares that column `NOT NULL`.
3. This app's own decoding, which has succeeded against production on every real Apple and Google
   sign-in and relaunch since 2026-09-11.

## Decision

These two responses are **constructed and cross-checked**, not captured. One constructed body per
response lives in `TestSupport/ConstructedResponse.swift`. It carries every key the Prisma model
and the source app's production models require, and every test that needs one of these responses
reads it. It is Swift, not a file under `Fixtures/`, because a file there claims to be a capture.
The fixtures README lists both as constructed.

Every other response keeps the original rule: a real capture via `Tools/capture-fixtures.sh`.

## Consequences

- There is no longer a guard that fails when the server changes these two shapes. Before, the
  constructed bodies were three hand-written copies that could drift apart; now there is one
  copy, and its comment names what it was checked against.
- The source app is being read for "what the API returns", which `AGENTS.md` allows. Its models
  count as evidence of the wire shape, not as a pattern for our types.
- The auth step 5 acceptance criterion "redacted live captures" is replaced by this one.
- A proxy is no longer part of any step, so the redaction question it raised does not arise.

## What would change this

Any of these reopens it:
- A production decoding failure on either route.
- A server change to `User` or `Tokens` that the source app's models did not see.
- A working capture path, such as a trusted proxy on a device, or a Debug-only log of raw auth
  bodies. With one, the constructed bodies are replaced by captures and this ADR is superseded.

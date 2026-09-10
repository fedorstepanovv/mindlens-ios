# ADR 0007 — Hand-written API client with captured-fixture contract tests

Date: 2026-09-10 · Status: Accepted

## Context
The backend has no OpenAPI document. `docs/API.md` is a hand-written 210-line contract
derived from reading NestJS controllers and the Prisma schema.

Nothing connects the two repositories. Rename a field server-side and nothing fails until
a user hits it. `CLAUDE.md` says "keep it current or it becomes a lie" — enforcement by
discipline, which is precisely what failed in the Flutter repo.

We also **own** the server, so generating a spec is available: `@nestjs/swagger` produces
an OpenAPI document from decorators the DTOs largely already carry, and Apple's
`swift-openapi-generator` turns that into a compile-checked client as an SPM build plugin.
That converts "keep this markdown current" into "the build fails when the server changes."

## Decision
Keep the hand-written client. Make drift detectable rather than preventable, via real
captured fixtures and `Tools/capture-fixtures.sh`.

## Consequences

The networking layer stays ours and stays legible — an interviewer can read `APIClient`
and `TokenRefresher` and see the reasoning, which generated code would hide. No changes to
a live production server, and no codegen step to reconcile with the no-codegen rule that
`CLAUDE.md` inherited from the Flutter app's `freezed`/`json_serializable` pain.

The cost is real and should not be understated: this is **detection, not prevention**.
Drift surfaces only for endpoints that have fixtures, and only when someone re-captures.
Generation would have made the compiler enforce it.

Two things keep the cost bounded:
- Re-capturing is one command, not a copy-paste chore.
- Every response type has a decoding test, so a re-capture immediately shows what moved.

## What would change this
A second client (Android, web) needing the same contract — at which point a shared
generated spec pays for itself and the argument for hand-writing evaporates. Also: the
first time a shape change reaches a user because no fixture covered it.

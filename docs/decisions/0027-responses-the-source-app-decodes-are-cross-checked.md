# ADR 0027 — Responses the source app decodes are cross-checked, not captured

Date: 2026-09-24 · Status: Accepted · Widens ADR 0024 from two auth responses to every response the source app decodes

## Context

ADR 0024 let two auth responses be constructed and cross-checked because they could not be captured.
Every other response still needed a live capture through `Tools/capture-fixtures.sh`.

Auth step 7 needs `GET /baseline/latest` and `GET /goals`. Both can be captured, but only with an access token
from a real production account. There is no staging, so the capture depends on Fedir handing a session a
token. Some of these responses are only produced by a write that happens once per account: `POST /baseline`
returns 409 the second time.

A capture proves less than it seems to, and the other two readings prove more:
- The source app's models (`../app/mindlensapp`) decode these exact responses in production today, with
  `required` marking each key they depend on. That is evidence from every user's traffic, not from one request.
- The server's Prisma schema produces the row. The response interceptor wraps it in the envelope and changes
  nothing else.
- A capture is one account on one day. It does not guard against drift unless someone re-runs it, and nobody
  has re-run one since `mood_create_200.json` was left provisional.

Fedir had asked sessions before this one to read the shape from the source app's models. The request was never
written down, so each new session read the capture rule and asked for a token again.

## Decision

**A response the source app decodes in production is constructed and cross-checked, not captured.**
- There is one body per response, in `TestSupport/ConstructedResponse.swift`. It carries every key that the
  Prisma model or the source app's model requires.
- Its doc comment names both sources by file, so a reviewer can check each key.
- Every test that needs that response reads that one body.

It is still Swift, not a file under `Fixtures/`, because a file there claims to be a capture (ADR 0024).

A capture is still used where it costs nothing: unauthenticated routes and error envelopes. The source app
models errors loosely, and the existing error fixtures caught three different envelope shapes. The files
already captured stay.

A response the source app never decodes has no second reader. It still needs a capture.

## Consequences

- Fixtures no longer need a token or a production write. The step that needed Fedir now needs nobody.
- Two readers have to agree, and a reviewer can check them file by file. A body typed from memory is still
  the mistake, and the lane's "fixtures typed rather than captured" check now means "typed without naming
  its two sources".
- When the server changes a shape, the source app breaks first, in production. We learn about the change
  from that app, not from a failing test here. A capture had the same blind spot until someone re-ran it.
- `AGENTS.md`, `docs/TESTING.md`, the `feature-start`, `feature-done` and `lane-verification` skills, and the
  fixtures README change with this ADR.

## What would change this

- A production decoding failure on a cross-checked response. From then on that response is captured.
- The source app is retired or stops being maintained, and its models stop tracking the server.
- A staging backend or a disposable test account, so that capturing costs nothing again.

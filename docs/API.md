# API Contract

**This is the only written specification of the Mindlens backend.** The server
(NestJS, `../server/amfine`) has no OpenAPI/Swagger document. If this file drifts, the
iOS client has nothing to check itself against.

Keep it current. When you touch an endpoint, update it here in the same change.

Source of truth in the server repo: `src/**/**.controller.ts` (routes),
`src/**/dto/*.ts` (shapes), `prisma/schema.prisma` (entities).

---

## Ground rules

- **No version prefix.** Routes are bare: `/mood`, `/auth/apple`. There is no `/api`,
  no `/v1`, and no version negotiation mechanism. Don't invent one client-side.
- **Every response is enveloped.**
  ```json
  { "data": <payload>, "statusCode": 200, "success": true, "timestamp": "ISO-8601" }
  ```
  Errors:
  ```json
  { "data": null, "success": false, "error": { "statusCode": 400, "message": "…", "error": "Bad Request" }, "timestamp": "ISO-8601" }
  ```
  `error.message` is a **string or an array of strings** — class-validator returns arrays.
  Decode it as either.

  Verified against production, and the **three** shapes genuinely differ — the status
  field is not even named consistently:
  ```json
  // 401 — no `error` key *inside* `error`, only `message` + `statusCode`
  {"data":null,"success":false,"error":{"message":"Unauthorized","statusCode":401},"timestamp":"…"}
  // 400 — array message, plus `error`
  {"data":null,"success":false,"error":{"message":["…"],"error":"Bad Request","statusCode":400},"timestamp":"…"}
  // 422 — `status`, NOT `statusCode`, and no `error` key
  {"data":null,"success":false,"error":{"status":422,"message":"invalid token provided"},"timestamp":"…"}
  ```
  The 422 comes from a hand-built `UnprocessableEntityException` payload rather than
  Nest's default filter, which is why its key differs. **Never classify on the body's
  status field** — `statusCode` is simply absent from the 422. Classify on the HTTP
  status; the body is only good for a message.
  Captured fixtures live in `Packages/MindlensKit/Sources/TestSupport/Fixtures`.
- **Unknown request fields are rejected.** The server runs `whitelist` +
  `forbidNonWhitelisted`, so sending an extra key is a 400, not a silent drop. Encode
  exactly the documented fields. Confirmed live: an extra key returns
  `["property surprise should not exist"]`.
- **Auth is default-on.** Every route needs `Authorization: Bearer <accessToken>` unless
  listed as public.
- **Rate limits.** 100 req/min globally; `/auth/apple` and `/auth/google` 10/min;
  `/auth/refresh` 20/min. Expect 429 and back off.

## Authentication

The backend verifies **Firebase ID tokens**, not raw Apple/Google credentials. The flow:

```
Sign in with Apple/Google (native)
   → Firebase Auth SDK → Firebase ID token
   → POST /auth/apple | /auth/google
   → Mindlens JWT pair { accessToken, refreshToken }
```

The iOS app therefore needs the Firebase Auth SDK. (Open question in `STATE.md`: whether
this indirection is intentional or historical.)

**One Firebase project, and it is production's.** The server's Admin SDK verifies against a
single project (`firebase.projectId` in its env), and there is no dev backend — the Flutter app's
`.env` carries one `API_URL`, production. So the app's bundle ID must be registered as an iOS app
*in that project* and ship that project's `GoogleService-Info.plist`; a token from any other
project is a 422 `invalid token provided`, indistinguishable at the client from a bad token.
Learned the hard way on 2026-09-11 with a plist from a second project.

`/auth/apple` answers 422 with **two different messages** (`auth.service.ts`): `invalid token
provided` when `verifyIdToken` throws, and `email required for new Apple sign-in (Apple did not
provide one)` when a *new* account has no email in the token or the body. The client shows one
copy for both; the message is in `AppError.diagnostic` and the session log.

| Method | Path | Auth | Purpose |
|---|---|---|---|
| POST | `/auth/apple` | public | Exchange Firebase ID token for JWT pair |
| POST | `/auth/google` | public | Same, Google provider |
| POST | `/auth/refresh` | refresh token in `Authorization` | Rotate the token pair |
| POST | `/auth/logout` | JWT | End this device's session (204) |

**Request** (`/auth/apple`, `/auth/google`):
```json
{ "idToken": "…", "guid": "device-uuid", "deviceModel": "iPhone17,1", "timezone": "Europe/Kyiv" }
```
`/auth/apple` additionally accepts optional `email` for the private-relay case.

Validation, and it bites: `guid` and `deviceModel` are both **6–36 characters**, `timezone` must
be a real IANA identifier, and `email` must parse. The six-character floor is not academic —
`utsname.machine` is `arm64` on a simulator, five characters, a 400 on every sign-in until
`SystemDeviceModel` was taught to handle it.

**Response** `data`:
```json
{ "user": { "id": 1, "email": "…", "authProvider": "APPLE",
            "timezone": "Europe/Kyiv", "isOnboardingComplete": false,
            "googleSocialId": null, "appleSocialId": "…",
            "createdAt": "ISO", "updatedAt": "ISO" },
  "tokens": { "accessToken": "jwt", "refreshToken": "jwt" } }
```
`user` is the **whole Prisma `User` row**, not a curated response shape
(`SocialLoginResponseDto` declares `user: User`). So a column added to that table appears
here without a server code change — decode the fields we need and ignore the rest, and do
not treat this list as closed.

`email` is `NOT NULL` in the schema, so it is never absent — the private-relay case is
handled by the server *rejecting* the sign-in (422) rather than by returning a null email.

`POST /auth/refresh` returns `data` as the bare token pair — `{accessToken, refreshToken}`,
with no `user` wrapper.

### Token rules that will bite you

- Access token: **15 minutes**. Refresh token: **30 days**.
- **Refresh is single-use and rotating.** `POST /auth/refresh` deletes the old session
  and issues a brand-new pair. Using a consumed refresh token fails permanently.
  → Concurrent 401s **must** be single-flighted. See ADR 0004.
- Sessions live in Redis, capped at **5 devices per user**, LRU-evicted. Re-login from
  the same `guid` replaces that device's session instead of consuming a new slot — so
  send a **stable, install-persistent device GUID** (Keychain, survives reinstall).
- JWT payload: `{ sub: userId (Int), tokenId: sessionId (UUID) }`.
- Refresh failure classification: **400/401/403 → session genuinely over, sign out.**
  Timeouts and 5xx → transient, keep the session and retry.

## Users, profile, onboarding

| Method | Path | Purpose |
|---|---|---|
| GET | `/users` | Current user row |
| DELETE | `/users` | Delete account (204) |
| PATCH | `/users/timezone` | Update timezone |
| POST | `/users/complete-onboarding` | Mark onboarding done |
| GET/POST/PATCH | `/profiles` | Read / create / update profile |
| GET | `/sessions` | List active device sessions |

`POST /profiles`:
```json
{ "firstName": "3-64 chars", "lastName": "3-64 chars",
  "gender": "MALE|FEMALE|OTHER", "dateOfBirth": "YYYY-MM-DD" }
```

## Core logging

| Method | Path | Purpose |
|---|---|---|
| POST | `/mood` | Log mood. Body `{ "moodRate": 1…4 }` |
| GET | `/mood?date=&timezone=` | Moods for a local day |
| PATCH/DELETE | `/mood/:id` | Update / delete |
| POST | `/daily-tags` | Attach tags to today |
| GET | `/daily-tags?date=&timezone=` | Tags for a date |
| DELETE | `/daily-tags/:id` | Detach (204) |
| GET/POST | `/categories`, `/tags` | Tag taxonomy (categories nest their tags) |
| POST | `/notes` | Create note |
| POST | `/notes/list` | Cursor-paginated list (filters in the **body**) |
| GET/PATCH/DELETE | `/notes/:id` | Single note |
| POST | `/events`, `/events/list` | Same shape as notes |

⚠️ The mood field is **`moodRate` in requests** but **`mood` in responses**. Not a typo —
model it explicitly so the asymmetry is visible rather than surprising.

`POST /notes/list` — pagination is a Prisma cursor, not offset:
```json
{ "cursor": "id of last note from previous page (omit for page 1)",
  "take": 10, "timezone": "Europe/Kyiv",
  "type": "COMMON|QUICK", "tags": ["uuid"], "createdAfter": "YYYY-MM-DD" }
```
→ `{ "notes": [...], "isLastPage": bool }`, ordered `createdAt desc`.

New users are auto-seeded with 6 tag categories: Body, Activity, Work, Social, Recovery,
Disruptors.

## Surveys and AI generation

| Method | Path | Purpose |
|---|---|---|
| GET | `/surveys?date=&timezone=` | Survey for a date, with questions + answers + insight |
| POST | `/surveys/submit` | Submit answers |
| GET | `/surveys/onboarding/status` | Poll onboarding generation progress |

**There is no streaming anywhere in this API** — no SSE, no websockets. LLM work runs in
BullMQ worker jobs and the client **polls**:

```json
{ "stage": "GENERATING_QUESTIONS|QUESTIONS_READY|GENERATING_INSIGHT|INSIGHT_READY",
  "status": "pending|ready",   // legacy, ignore
  "survey": null | { … } }
```

Poll with backoff, and give the user something to look at — generation is genuinely slow.

`POST /surveys/submit` is **idempotent-hostile**: submitting a completed survey is a 400.

## Insights and recaps

| Method | Path | Purpose |
|---|---|---|
| GET | `/analytics/summary?period=WEEKLY\|MONTHLY&date=&timezone=` | Recap, or `null` |

Returns `null` — not 404 — until the background job has produced a row. Treat `null` as
"not ready yet", not as an error. Payload carries `generatedInsights[]` and `anomalies[]`
alongside mood/sleep/steps aggregates.

## Focus areas (the "Lens")

| Method | Path | Purpose |
|---|---|---|
| GET | `/focus-area/active` | Current active focus area |
| GET | `/focus-area/candidates?limit=` | Candidate deck (1–50, default 20) |
| POST | `/focus-area/mark-as-active` | Promote a candidate |
| POST | `/focus-area/resolve` | Resolve as completed/failed |
| POST | `/focus-area/reject` | Reject a candidate |

## Health, goals, baseline, notifications

| Method | Path | Purpose |
|---|---|---|
| POST/GET/PATCH/DELETE | `/health/steps`, `/health/bpm`, `/health/sleep` | Health records; `source: "OS"\|"MANUAL"` |
| GET/POST/DELETE | `/goals` | Goals |
| POST | `/baseline`, GET `/baseline/latest` | Onboarding baseline (404 if none) |
| GET/POST/PATCH/DELETE | `/notification-settings[/daily[/:id]]` | Reminder times, `"HH:mm"` |

## Subscriptions

| Method | Path | Purpose |
|---|---|---|
| POST | `/subscriptions/webhook` | RevenueCat → server. **Not for the client.** |

**There is no client-facing subscription endpoint.** `GET /users` does not include
entitlement. Subscription state comes from the **RevenueCat SDK on-device**; the server
only tracks it internally for job scheduling.

## Push notifications

No endpoint registers a push token — OneSignal's SDK handles it on-device.

⚠️ **The external ID must be exactly `mindlens-user-<userId>`.** The server addresses
users by that prefixed string. The prefix exists because the OneSignal mobile SDK
silently drops purely numeric external IDs — so getting this wrong produces no error and
no notifications. From the server's own comment: *"Keep this in sync with the mobile
client."*

## Environments

Production: `https://mindlens-api-production.up.railway.app`

Set via `API_BASE_URL` in `Config/Base.xcconfig`, which **is** committed — a public
endpoint is not a credential, and git-ignoring it would break a fresh clone. Actual
secrets go in `Config/Secrets.xcconfig` (git-ignored, see `Config/Secrets.example.xcconfig`).

There is no staging environment. Development runs against production, so be deliberate
about writes.

## Status codes in use

`200` · `204` (deletes, logout) · `400` (validation, unknown fields) · `401` (bad/expired
JWT) · `403` (dev-only routes in prod) · `404` · `409` (unique conflict) · `422` (invalid
Firebase token, missing Apple email) · `429` (throttled) · `500`

# Fixtures

Real responses captured from the production API, used to prove our `Codable` types match
what the server actually sends. There is no OpenAPI spec — these are the contract guard.

| File | Captured | Notes |
|---|---|---|
| `error_unauthorized_401.json` | ✅ live | Note there is **no** `error` key inside `error` — only `message` + `statusCode` |
| `error_validation_400.json` | ✅ live | `message` is an **array** (class-validator) |
| `error_unknown_field_400.json` | ✅ live | Proves the server rejects unknown request fields |
| `error_invalid_token_422.json` | ✅ live | A rejected Firebase token. Carries **`status`**, not `statusCode`, and no `error` key — a third envelope shape |
| `health_ready_200.json` | ✅ live | Success envelope |
| `mood_create_200.json` | ⚠️ provisional | Hand-written from the Prisma schema. **Replace with a real capture once auth works.** |
| *`POST /auth/apple` 200* | ◇ constructed | Not a file here: `ConstructedResponse.signIn`. It cannot be captured, so it is cross-checked against Prisma and the source app's production models (ADR 0024) |
| *`POST /auth/refresh` 200* | ◇ constructed | Not a file here: `ConstructedResponse.tokenRefresh`. Same reason and ADR |

To capture a new one:

```
curl -sS https://mindlens-api-production.up.railway.app/<path> \
  -H "Authorization: Bearer $TOKEN" -o <name>.json
```

Never hand-write a fixture. A hand-written fixture asserts what we hoped the server does.

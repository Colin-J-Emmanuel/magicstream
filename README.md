# MagicStream

A full-stack movie streaming app with AI-ranked reviews and personalized recommendations. Users browse a movie catalog and watch trailers; an admin writes reviews, an LLM classifies each review into a fixed ranking, and users get recommendations from their favorite genres, sorted best-first.

**Stack:** Go (Gin) · MongoDB · React · OpenAI-compatible LLM (Groq free tier) · Docker Compose

> **Status:** 🚧 In active development. The data layer (server, database connection, data model, seeding, read API) is complete and tested; authentication is in progress. See the [Build log](#build-log) for what's done and the [Roadmap](#roadmap) for what's next.

---

## Table of contents

- [Architecture](#architecture)
- [Getting started](#getting-started)
- [Configuration](#configuration)
- [API](#api)
- [Project structure](#project-structure)
- [Design decisions](#design-decisions)
- [Build log](#build-log)
- [Roadmap](#roadmap)

---

## Architecture

```
┌──────────────┐   HTTP + http-only cookies   ┌──────────────────┐      ┌───────────┐
│ React client │ ───────────────────────────▶ │  Go / Gin API    │ ───▶ │  MongoDB  │
│  (Vercel)    │ ◀─────────────────────────── │  (Render)        │      │  (Atlas)  │
└──────────────┘                              └────────┬─────────┘      └───────────┘
                                                       │ on review write only
                                                       ▼
                                          ┌──────────────────────────┐
                                          │ LLM (OpenAI-compatible)  │
                                          │ Groq · gpt-oss-20b       │
                                          └──────────────────────────┘
```

The LLM is called **only when an admin saves a review**, never when a user browses or requests recommendations. Rankings are computed once and stored, so the read path is a plain database query.

---

## Getting started

### Prerequisites

- [Go](https://go.dev/dl/) 1.27+
- [Docker Desktop](https://www.docker.com/products/docker-desktop/) (runs MongoDB locally)
- A free [Groq API key](https://console.groq.com/) (no credit card required)

### Run locally

```bash
git clone https://github.com/Colin-J-Emmanuel/magicstream.git
cd magicstream

# 1. Start MongoDB
docker compose up -d

# 2. Configure the server
cd server
cp .env.example .env        # then add your LLM_API_KEY and two token secrets
                            # (generate each with: openssl rand -base64 48)

# 3. Load sample movies (safe to re-run)
go run ./cmd/seed

# 4. Run the API
go run .
```

### Verify

```bash
curl -s localhost:8080/health
# {"database":"ok","status":"ok"}
```

### Run unit tests

```bash
cd server
go test ./...
```

---

## Configuration

All configuration is read from environment variables (`server/.env` locally; the platform's settings in production). `server/.env.example` is the committed template.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `MONGODB_URI` | yes | — | MongoDB connection string |
| `DATABASE_NAME` | yes | — | Database name (`magicstream`) |
| `PORT` | no | `8080` | HTTP port (set automatically by Render) |
| `ACCESS_TOKEN_SECRET` | yes | — | Signs access tokens. At least 32 characters |
| `REFRESH_TOKEN_SECRET` | yes | — | Signs refresh tokens. At least 32 characters, and must differ from the access secret |
| `COOKIE_SECURE` | no | `false` | Set `true` in production so auth cookies are sent only over HTTPS |
| `LLM_API_KEY` | yes* | — | API key for the LLM provider |
| `LLM_BASE_URL` | yes* | — | OpenAI-compatible endpoint, e.g. `https://api.groq.com/openai/v1` |
| `LLM_MODEL` | yes* | — | Model name, e.g. `openai/gpt-oss-20b` |

\* Used from Phase 3 onward.

---

## API

| Method | Path | Description | Status |
|---|---|---|---|
| `GET` | `/health` | Liveness + database reachability. `200` when healthy, `503` when MongoDB is unreachable | ✅ |
| `GET` | `/movies` | List all movies, sorted by title. Returns `[]` (never `null`) when empty | ✅ |
| `GET` | `/movies/:imdb_id` | Get one movie. `400` if the ID isn't IMDb-shaped, `404` if not found | ✅ |
| `POST` | `/auth/register` | Create an account. `201` on success, `400` on invalid input, `409` if the email is taken | ✅ |
| `POST` | `/auth/login` | Verify credentials and set `access_token` / `refresh_token` http-only cookies. `401` with one generic message for any bad credentials | ✅ |
| `POST` | `/auth/refresh` | Exchange the refresh cookie for a new token pair (rotation). Reusing an old refresh token revokes the whole session | ✅ |
| `POST` | `/auth/logout` | Revoke the session and clear both cookies. Always `204`, even with no cookie | ✅ |
| `GET` | `/me` | 🔒 The current user, read fresh from the database. `401` without a valid access token | ✅ |
| `PATCH` | `/movies/:imdb_id/review` | Admin review → LLM ranking | Phase 3 |
| `GET` | `/recommendations` | Movies from the user's favorite genres, best-ranked first | Phase 4 |

🔒 = requires authentication.

---

## Project structure

```
magicstream/
├── docker-compose.yml        # Local MongoDB with a persistent named volume
├── client/                   # React app (Phase 5)
└── server/
    ├── main.go               # Entry point: config, DB connection, indexes, routes
    ├── auth/
    │   ├── tokens.go         # JWT issuing and verification, separate access/refresh secrets
    │   └── tokens_test.go    # Unit tests (expired-token rejection)
    ├── cmd/
    │   └── seed/
    │       └── main.go       # Idempotent seed script: go run ./cmd/seed
    ├── seed/
    │   └── movies.json       # Catalog data owned by the seed script
    ├── database/
    │   └── database.go       # Mongo connection (fail-fast ping) and index setup
    ├── handlers/
    │   ├── auth.go           # Register, login, refresh, logout, /me
    │   └── movies.go         # Movie read endpoints
    ├── middleware/
    │   └── auth.go           # RequireAuth: verifies the access cookie, sets user identity
    ├── models/
    │   ├── movie.go          # Movie, Genre, Ranking document types
    │   ├── refresh_token.go  # Server-side refresh token record (hash only)
    │   └── user.go           # User document and RegisterRequest
    ├── .env.example          # Committed config template
    ├── go.mod
    └── go.sum
```

---

## Design decisions

Each decision records what was chosen and **why**, over the alternatives.

### LLM provider is configuration, not code
The server talks to any OpenAI-compatible endpoint through three env vars (`LLM_API_KEY`, `LLM_BASE_URL`, `LLM_MODEL`). It runs on Groq's free tier today, serving OpenAI's open-weight `gpt-oss-20b`. Switching to OpenAI, Gemini, or another provider is a deployment change with no code change. Variable names are deliberately provider-neutral (`LLM_API_KEY`, not `GROQ_API_KEY`) so the code never encodes a vendor.

### Classify reviews at write time, not read time
The LLM ranks a review once, when the admin saves it, and the result is stored. Recommendations are then a pure database query with zero LLM calls. This keeps reads fast and cheap, and it is what makes a rate-limited free tier viable: LLM traffic scales with *admin writes*, not *user reads*.

### Fail fast on startup
`mongo.Connect` is lazy and doesn't open a connection. The server explicitly pings MongoDB at startup with a 5-second timeout and exits with a clear error if it can't reach it. A misconfigured URI surfaces immediately instead of on the first user request.

### Health check reports database reachability
`/health` pings MongoDB with a 2-second timeout and returns `503` if it fails. A process that's running but can't reach its database isn't healthy, and the hosting platform should know. The Phase 0 version returned `200` while MongoDB was down; the failure-path test caught it.

### Errors are wrapped with context
Errors are wrapped at each layer with `fmt.Errorf("...: %w", err)`, so a single log line traces the failure to its root cause, e.g. `database connection failed: pinging mongo: context deadline exceeded ... connection refused`.

### No trusted proxies by default
`router.SetTrustedProxies(nil)` stops Gin from trusting `X-Forwarded-For` headers from arbitrary clients, which would let a client spoof its IP. This will be revisited for production, where the API sits behind Render's proxy.

### `imdb_id` is the natural key, enforced by a unique index
MongoDB assigns every document an `_id`, but a movie's real-world identity is its IMDb ID. A unique index on `imdb_id` makes duplicate movies impossible at the database level, regardless of application code. Index creation runs on every startup and is idempotent: creating an index that already exists with the same definition is a no-op, so no separate migration step is needed.

### Genres are embedded, not referenced
Genres are small, rarely change, and are always read together with their movie. Embedding them avoids a second query (or a join-like `$lookup`) on every read. The trade-off, accepted deliberately: renaming a genre means updating every movie that has it, which is acceptable for a fixed list of about ten genres.

### Ranking stores both a value and a name
`ranking_value` (1 = Excellent … 5 = Terrible) exists so recommendations can be **sorted in the database**. MongoDB can only sort by a stored field, so mapping names to numbers in Go would mean fetching every matching movie and sorting in application code. `ranking_name` is for display. Unranked movies get a sentinel value of `999` so they sort last.

### Seeding is idempotent, with split field ownership
The seed script upserts each movie by `imdb_id`, so it can run any number of times: new movies are inserted, edited catalog data is updated, and unchanged movies are left alone. The key decision is **who owns each field**:

- **Catalog fields** (`title`, `poster_path`, `youtube_id`, `genre`) are owned by `seed/movies.json` and written with `$set` on every run.
- **Review fields** (`admin_review`, `ranking`) are owned by the app and written with `$setOnInsert`, only when a movie is first created.

A naive seed that overwrites whole documents would wipe every admin review on the next reseed. Splitting ownership lets the seed file stay the source of truth for the catalog without ever touching work done through the app.

### Handlers receive dependencies explicitly
Each group of routes is a struct (`MovieHandler`) constructed with the collection it needs, rather than reading a global database variable. Dependencies are visible in the constructor, and a handler can be tested in isolation by passing in a test collection.

### Distinct errors, generic messages
`GET /movies/:imdb_id` distinguishes a malformed ID (`400`) from a missing movie (`404`), so clients can tell bad input from absent data. Malformed IDs are rejected before reaching MongoDB: in testing, a `400` took ~50µs against ~6ms for a `404` that queried the database. Unexpected database errors are logged in full on the server, but clients receive only a generic message, so internal details never leak.

### Empty lists encode as `[]`, not `null`
In Go, a nil slice JSON-encodes as `null`. The list handler initializes an empty slice so an empty collection returns `[]`, and the React client can always call `.map()` safely.

### Passwords are hashed with bcrypt at cost 12
Passwords are hashed, never encrypted: a hash can't be reversed, only checked against a guess. bcrypt is deliberately slow (~350ms per hash at cost 12) and automatically salted, which makes brute-forcing a stolen database impractical and precomputed lookup tables useless. bcrypt only reads the first 72 *bytes* of a password, while validation counts *characters*, so passwords over 72 bytes are rejected explicitly rather than silently truncated.

### Password hashes can never appear in a response
`User.PasswordHash` carries the struct tag `json:"-"`, so it is excluded from JSON encoding entirely. Even a handler that returns a whole `User` cannot leak it: the guarantee lives in the type, not in each handler remembering to strip it.

### Clients cannot assign themselves a role
Registration decodes into `RegisterRequest`, a separate type with no `Role` field. A client that sends `"role": "ADMIN"` is ignored, and the server always sets `USER`. This closes the mass-assignment vulnerability structurally: the attack can't be expressed, rather than being checked for at runtime.

### Email uniqueness is the database's job
Emails are lowercased and trimmed, then inserted directly. A unique index rejects duplicates, and the duplicate-key error becomes a `409`. Checking for an existing email before inserting would be racy (two simultaneous signups can both pass the check), so the index is the only source of truth. The trade-off: a duplicate signup still pays for a bcrypt hash before the insert fails, an acceptable cost for race-free code on a rare path.

### Two tokens: short-lived access, revocable refresh
An access token (15 minutes) is verified by its signature alone, with no database lookup, which makes every authenticated request cheap. The cost is that it can't be revoked before it expires, so its lifetime is kept short. A refresh token (7 days) is used only to obtain new access tokens, and will be checked against the database so it can be revoked on logout.

### JWTs carry only what is safe to read
A JWT is signed, not encrypted: anyone holding one can decode its payload. The signature only prevents *changes*. So the payload holds just the user ID, role, and timestamps. No email, no password hash, nothing sensitive.

### Separate secrets for access and refresh tokens
If both token types shared one signing key, a stolen 7-day refresh token would also pass as an access token. With separate secrets, that confusion is cryptographically impossible: no runtime "token type" check needs to be remembered. The server refuses to start if either secret is missing, shorter than 32 characters, or identical to the other.

### Login reveals nothing about which accounts exist
An unknown email and a wrong password both return the same `401 invalid email or password`. Because a wrong password costs ~350ms of bcrypt while a missing account would otherwise return instantly, the handler also runs bcrypt against a dummy hash when the email isn't found. In testing, the two cases took 340ms and 346ms: indistinguishable by message or by timing. (Registration's `409` does reveal whether an email is taken, a common trade-off for signup UX.)

### Tokens live in http-only cookies with a narrow refresh path
Tokens are set as `HttpOnly` cookies so injected JavaScript can never read them, unlike `localStorage`. `SameSite=Lax` is a first layer of CSRF defense, and `Secure` is enabled in production. The refresh cookie's `Path` is `/auth`, so the long-lived token is sent only to authentication routes, never with ordinary requests like `/movies`.

### Tokens get no say in how they're verified
A JWT's header declares its own signing algorithm, and early libraries trusted it: an attacker could set `"alg": "none"`, drop the signature, and be accepted. Verification here pins the algorithm to HS256 and supplies the key itself, ignoring whatever the token claims. An expiry claim is also required, so a token minted without one is rejected rather than treated as valid forever. All three attack paths were tested, and each was caught by a different defense: a tampered payload by the signature check, `alg: none` by the algorithm pin, and a refresh token posing as an access token by the separate secrets.

### Protection is applied by route group, not per route
`RequireAuth` is attached to a route group, and protected endpoints are registered on that group. A new protected endpoint can't accidentally ship without the middleware. The middleware stops the chain with `AbortWithStatusJSON`, so a rejected request can never fall through to the handler.

### Handlers never touch tokens
The middleware verifies the token and stores the user ID and role on the request context; handlers read them through `middleware.UserID(c)` and `middleware.Role(c)`. Token handling lives in exactly one place.

### `/me` reads the database, not the token
A JWT is a snapshot taken at login: a role changed or an account deleted since then isn't reflected in it. `/me` uses the token only to identify the caller, then loads the current user from MongoDB. A deleted account gets a `401`.

### One response for every authentication failure
A missing cookie, a bad signature, a forged algorithm, and an expired token all return the same `401 authentication required`. The specific reason is logged on the server for debugging; a client probing the API learns nothing about which check it failed.

### Refresh tokens are stored server-side, as SHA-256 hashes
Revocation requires state, so every issued refresh token has a database record. The record holds a SHA-256 hash of the token, never the token itself, so a database leak yields nothing that can be replayed as a cookie. SHA-256 rather than bcrypt is deliberate: bcrypt's slowness protects low-entropy, human-chosen passwords, while a refresh token is ~256 bits of server-generated randomness with nothing to guess. A fast hash is safe here, and a refresh takes ~14ms instead of ~350ms. Every token also carries a random `jti` claim, so two tokens issued in the same second can never be byte-identical.

### Rotation with reuse detection
Every refresh token is single-use: `/auth/refresh` marks it used and issues a new pair in the same session *family*. A legitimate client only ever presents its latest token, so a token that has already been used appearing again means two parties hold copies: the user and a thief. The server can't tell which request is which, so it revokes the entire family, logging both out. A stolen refresh token becomes worthless the moment either party uses it after the other. Used tokens are kept (marked, not deleted) until their natural expiry precisely so reuse can be recognized.

**Known limitation:** two browser tabs refreshing at the same instant with the same token will trigger a false reuse alarm and log the user out. Production systems add a short grace window for this; it is noted here rather than implemented.

### Claiming a refresh token is atomic
A check-then-update sequence would let two simultaneous requests both spend the same token. Instead, a single `FindOneAndUpdate` filtered on `used_at: null` finds the token and marks it used in one operation, and MongoDB guarantees only one request can win.

### Expired records clean themselves up
A TTL index on `expires_at` makes MongoDB delete each refresh-token record automatically once it expires. No cleanup job exists to forget about or to fail.

### Logout is idempotent, with an honest limit
Logout revokes the session's whole family and clears both cookies, and it always returns `204`, because "make sure I'm logged out" should never fail. It doesn't verify the refresh JWT's signature: it hashes what it received and looks it up, so forged tokens simply match nothing, and a session can be revoked even after its JWT expires. Cookies are cleared on the same `Path` they were set with; otherwise the browser would keep them. **Limit:** an access token issued before logout remains valid until it expires (at most 15 minutes), because access tokens are verified without the database. This was tested and confirmed, and it is the deliberate cost of stateless access tokens.

---

## Build log

Each phase is decomposed into individually tested bricks. Every brick is verified on both the happy path and the failure path before moving on.

### Phase 0 — Foundation ✅

- Go module, Gin server with `GET /health`
- MongoDB 7 via Docker Compose, with a named volume so data survives restarts
- `.gitignore` and `.env.example`; verified with `git check-ignore` that `.env` is never tracked
- LLM smoke test against Groq's OpenAI-compatible endpoint

| Test | Expected | Result |
|---|---|---|
| `GET /health` | `200` | ✅ |
| `GET /nope` (unknown route) | `404` | ✅ |
| Mongo `ping` | `{ ok: 1 }` | ✅ |
| LLM: positive review | `Excellent` | ✅ |
| LLM: negative review | `Bad` or `Terrible` | ✅ `Terrible` |
| LLM: mixed review | `Okay` or `Good` | ✅ `Okay` |

### Phase 1a — Database connection ✅

- `database.Connect`: creates the client and pings with a 5s timeout
- `/health` now checks database reachability with a 2s timeout
- Environment loading via `godotenv`; clean disconnect on shutdown

| Test | Expected | Result |
|---|---|---|
| Health, Mongo up | `200`, `database: ok` | ✅ |
| Health, Mongo stopped | `503`, `database: unreachable` after 2s | ✅ |
| Mongo restarted, server untouched | `200` again (pool reconnects automatically) | ✅ |
| Server start with Mongo down | Exits within ~5s with a wrapped error | ✅ |

### Phase 1b — Movie model and unique index ✅

- `Movie`, `Genre`, `Ranking` types with `bson`/`json` struct tags
- `database.EnsureIndexes`: unique index on `movies.imdb_id`, run at startup

| Test | Expected | Result |
|---|---|---|
| `getIndexes()` | `imdb_id_1` with `unique: true` | ✅ |
| Restart server | Starts cleanly (index creation is idempotent) | ✅ |
| Insert duplicate `imdb_id` | `E11000 duplicate key error` | ✅ |

### Phase 1c — Idempotent seed script ✅

- `cmd/seed`: a second executable in the same module, reusing the `database` and `models` packages
- Upsert by `imdb_id`; `$set` for catalog fields, `$setOnInsert` for review fields
- Reports inserted / updated / unchanged counts per run

| Test | Expected | Result |
|---|---|---|
| First run | `8 inserted, 0 updated, 0 unchanged` | ✅ |
| Second run (idempotency) | `0 inserted, 0 updated, 8 unchanged` | ✅ |
| Set an admin review, then reseed | Review and ranking survive | ✅ |
| Edit one title in the seed file, reseed | `0 inserted, 1 updated, 7 unchanged` | ✅ |
| Revert the title, reseed | `0 inserted, 1 updated, 7 unchanged` | ✅ |

### Phase 1d — Read endpoints ✅

- `MovieHandler` with `List` and `Get`, constructed with its collection
- IMDb ID format validation (`^tt\d{7,8}$`) before any database call
- 5-second query timeouts derived from the request context

| Test | Expected | Result |
|---|---|---|
| `GET /movies` | `200`, 8 movies sorted by title | ✅ |
| `GET /movies/tt0111161` | `200`, full document | ✅ |
| `GET /movies/tt9999999` | `404 movie not found` | ✅ |
| `GET /movies/not-an-id` | `400 invalid imdb_id format` | ✅ |
| `GET /movies` on an empty collection | `[]`, not `null` | ✅ |

### Phase 2a — Registration ✅

- `User` model with a hidden password hash; separate `RegisterRequest` with validation tags
- Unique index on `users.email`, created at startup alongside the movies index
- `POST /register`: validate, hash with bcrypt (cost 12), normalize email, insert

| Test | Expected | Result |
|---|---|---|
| Valid registration | `201`, email lowercased, role `USER`, no `password_hash` in body (~350ms) | ✅ |
| Same email, different case | `409` | ✅ |
| Body includes `"role":"ADMIN"` | `201` with role `USER` | ✅ |
| Password under 8 characters | `400`, fails `min` tag | ✅ |
| Malformed email | `400`, fails `email` tag | ✅ |
| Missing required fields | `400`, lists each missing field | ✅ |
| Stored hash | Begins `$2a$12$` | ✅ |

### Phase 2b — Login ✅

- `auth.TokenManager`: HS256 JWTs with separate access/refresh secrets, validated at startup
- `POST /auth/login`: generic error message, timing equalized with a dummy bcrypt hash
- Auth routes grouped under `/auth`; tokens set as `HttpOnly`, `SameSite=Lax` cookies

| Test | Expected | Result |
|---|---|---|
| Start with a short secret | Server refuses to start with a clear error | ✅ |
| `POST /register` (old path) | `404` | ✅ |
| Valid login | `200` and two cookies | ✅ |
| Cookie attributes | Both `HttpOnly`, `SameSite=Lax`; access `Path=/`, 15 min; refresh `Path=/auth`, 7 days | ✅ |
| Wrong password | `401 invalid email or password` (~340ms) | ✅ |
| Unknown email | Same `401` message, same timing (~346ms) | ✅ |
| Decode access token payload | Only `sub`, `role`, `iat`, `exp`; `exp − iat` = 900s | ✅ |

### Phase 2c — Auth middleware ✅

- `TokenManager.ParseAccess` / `ParseRefresh`: HS256 pinned, expiry required
- `middleware.RequireAuth`: reads the `access_token` cookie, verifies it, sets user ID and role on the context
- Protected route group with `GET /me`
- First unit test: `go test ./auth/`

| Test | Expected | Server log reason | Result |
|---|---|---|---|
| No cookie | `401` | (not logged: ordinary anonymous traffic) | ✅ |
| Valid token | `200`, current user, no `password_hash` | — | ✅ |
| Payload tampered to `role: ADMIN` | `401` | `token signature is invalid` | ✅ |
| Header forged to `alg: none`, signature removed | `401` | `signing method none is invalid` | ✅ |
| Refresh token sent as access token | `401` | `token signature is invalid` | ✅ |
| Expired token (unit test) | `ErrTokenExpired` | — | ✅ |

### Phase 2d — Refresh rotation and logout ✅

- `refresh_tokens` collection: SHA-256 hash, user, family, expiry, `used_at`; unique, family, and TTL indexes
- `POST /auth/refresh`: atomic claim, rotation within the family, reuse detection that revokes the family
- `POST /auth/logout`: revokes the family, clears cookies on their original paths, idempotent
- Every token carries a random `jti`

| Test | Expected | Result |
|---|---|---|
| Refresh with a valid token | `200`, new pair, refresh token changed (~14ms, no bcrypt) | ✅ |
| Database after rotation | One family, two hashed records: old `used_at` set, new `null` | ✅ |
| Replay the old (used) token | `401`, server logs reuse detected and revokes the family | ✅ |
| Then use the legitimate new token | `401`: the whole family is revoked | ✅ |
| Records remaining for that family | `0` | ✅ |
| Logout | `204`, both cookies `Max-Age=0` on their original paths | ✅ |
| Refresh with a token saved before logout | `401`, no reuse alarm (revoked, not reused) | ✅ |
| Access token saved before logout | `200` until expiry: the documented trade-off | ✅ |
| Logout with no cookies | `204` (idempotent) | ✅ |
| Unit tests after token changes | Pass | ✅ |

---

## Roadmap

| Phase | Scope | Status |
|---|---|---|
| 0 | Foundation: server, Docker Compose MongoDB, config, LLM smoke test | ✅ |
| 1 | Data layer: connection, movie model, unique index, idempotent seeding, read endpoints | ✅ |
| 2 | Auth: registration (bcrypt), login, access/refresh JWTs in http-only cookies, middleware | 🔨 2a–2d done |
| 3 | Admin review → LLM ranking, with strict output validation and retry on rate limits | ⬜ |
| 4 | Recommendations by favorite genres, sorted by ranking | ⬜ |
| 5 | React client: browse, auth, trailer player, recommendations, admin review form | ⬜ |
| 6 | Deploy: MongoDB Atlas, API on Render, client on Vercel | ⬜ |

---

## Author

**Colin Emmanuel** · [GitHub](https://github.com/Colin-J-Emmanuel)
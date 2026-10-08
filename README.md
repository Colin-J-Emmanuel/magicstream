# MagicStream

A full-stack movie streaming app with AI-ranked reviews and personalized recommendations. Users browse a movie catalog and watch trailers; an admin writes reviews, an LLM classifies each review into a fixed ranking, and users get recommendations from their favorite genres, sorted best-first.

**Stack:** Go (Gin) · MongoDB · React · OpenAI-compatible LLM (Groq free tier) · Docker Compose

> **Status:** 🚧 In active development. The backend foundation (server, database connection, data model, seeding) is complete and tested. See the [Build log](#build-log) for what's done and the [Roadmap](#roadmap) for what's next.

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
cp .env.example .env        # then add your LLM_API_KEY

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

---

## Configuration

All configuration is read from environment variables (`server/.env` locally; the platform's settings in production). `server/.env.example` is the committed template.

| Variable | Required | Default | Purpose |
|---|---|---|---|
| `MONGODB_URI` | yes | — | MongoDB connection string |
| `DATABASE_NAME` | yes | — | Database name (`magicstream`) |
| `PORT` | no | `8080` | HTTP port (set automatically by Render) |
| `LLM_API_KEY` | yes* | — | API key for the LLM provider |
| `LLM_BASE_URL` | yes* | — | OpenAI-compatible endpoint, e.g. `https://api.groq.com/openai/v1` |
| `LLM_MODEL` | yes* | — | Model name, e.g. `openai/gpt-oss-20b` |

\* Used from Phase 3 onward.

---

## API

| Method | Path | Description | Status |
|---|---|---|---|
| `GET` | `/health` | Liveness + database reachability. `200` when healthy, `503` when MongoDB is unreachable | ✅ |
| `GET` | `/movies` | List movies | Phase 1 |
| `GET` | `/movies/:imdb_id` | Get one movie | Phase 1 |
| `POST` | `/register`, `/login`, `/logout`, `/refresh` | Authentication | Phase 2 |
| `PATCH` | `/movies/:imdb_id/review` | Admin review → LLM ranking | Phase 3 |
| `GET` | `/recommendations` | Movies from the user's favorite genres, best-ranked first | Phase 4 |

---

## Project structure

```
magicstream/
├── docker-compose.yml        # Local MongoDB with a persistent named volume
├── client/                   # React app (Phase 5)
└── server/
    ├── main.go               # Entry point: config, DB connection, indexes, routes
    ├── cmd/
    │   └── seed/
    │       └── main.go       # Idempotent seed script: go run ./cmd/seed
    ├── seed/
    │   └── movies.json       # Catalog data owned by the seed script
    ├── database/
    │   └── database.go       # Mongo connection (fail-fast ping) and index setup
    ├── models/
    │   └── movie.go          # Movie, Genre, Ranking document types
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

---

## Roadmap

| Phase | Scope | Status |
|---|---|---|
| 0 | Foundation: server, Docker Compose MongoDB, config, LLM smoke test | ✅ |
| 1 | Data layer: connection, movie model, unique index, idempotent seeding, read endpoints | 🔨 1a–1c done |
| 2 | Auth: registration (bcrypt), login, access/refresh JWTs in http-only cookies, middleware | ⬜ |
| 3 | Admin review → LLM ranking, with strict output validation and retry on rate limits | ⬜ |
| 4 | Recommendations by favorite genres, sorted by ranking | ⬜ |
| 5 | React client: browse, auth, trailer player, recommendations, admin review form | ⬜ |
| 6 | Deploy: MongoDB Atlas, API on Render, client on Vercel | ⬜ |

---

## Author

**Colin Emmanuel** · [GitHub](https://github.com/Colin-J-Emmanuel)
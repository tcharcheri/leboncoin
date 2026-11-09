# Way of working

This repository is read-only, click on "Use this template" => "Create a new repository" and work on your copy just as you would on a live one. \
Once you are satisfied with your work, please send your public repository url to the hiring manager.

# Hiring instructions

Please refer to [these instructions](INSTRUCTIONS.md).

# Ads API

A small Go HTTP API around a single domain: *ads* (classified ads).

The API needs an up-and-running postgres database, which can be launched via the provided Docker Compose file.

## Requirements

- Go 1.26 or later
- Docker with the Compose plugin

## Launching the stack

```bash
docker compose up -d
```

This starts PostgreSQL with the schema and seed data already applied, as well as the `ads` api.

## Model

There are 2 entities in this sample:
- ad (title, price, etc...)
- user (id and nickname only)

## API

| Method   | Path          | Auth | Description                                                                                          |
|----------|---------------|------|------------------------------------------------------------------------------------------------------|
| `GET`    | `/ads`        | no   | List ads, newest first (filters `title`, `user_id`, `min_price_cents`, `max_price_cents`; `limit`, `offset`) |
| `GET`    | `/ads/{id}`   | no   | Get a single ad, with its owner's `username`                                                         |
| `POST`   | `/ads`        | yes  | Create an ad (5 per rolling 24 hours)                                                                |
| `DELETE` | `/ads/{id}`   | yes  | Delete an ad (owner only)                                                                            |
| `POST`   | `/ads/import` | yes  | Bulk ad import, atomic and idempotent with an `Idempotency-Key` header                               |
```bash
# Health check
curl -s -v http://localhost:8080/health

# List ads (with optional filters)
curl -s 'http://localhost:8080/ads?title=bike&min_price_cents=1000'

# Get a single ad
curl -s http://localhost:8080/ads/1

# Create an ad (requires Authorization header)
curl -s -X POST http://localhost:8080/ads \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Mountain bike","price_cents":12000,"photo_url":"https://example.com/photos/bike.jpg"}'
  
  # Create an ad with a price in euros (preferred; price_cents is deprecated)
curl -s -X POST http://localhost:8080/ads \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Mountain bike","price":120.50,"photo_url":"https://example.com/photos/bike.jpg"}'
  
  # Bulk import, safe to retry: reuse the same Idempotency-Key when retrying
curl -s -X POST http://localhost:8080/ads/import \
  -H 'Authorization: 00000000-0000-0000-0000-000000000001' \
  -H 'Idempotency-Key: 6f1d2c9e-7a3b-4e8f-9c1d-2b3a4c5d6e7f' \
  -H 'Content-Type: application/json' \
  -d '{"ads":[{"title":"Chair","price":15},{"title":"Table","price":80}]}'
```

## Authentication

To pass authentication information to a call, simply add the `Authorization` header with the user ID in plain text.

It will be extracted by the service.

```
Authorization: 00000000-0000-0000-0000-000000000001
```

## Tests

To launch all the tests:

```shell
go test ./...
```

## Design decisions Very important

### `price_cents` → `price` (float)

Renaming the field to a float was not applied as-is:

- **Floats are unsafe for money**: `0.1 + 0.2 != 0.3` in binary floating point, which leads to rounding errors in totals, comparisons and filters.
- **Renaming breaks existing clients**: mobile apps already installed keep reading `price_cents` and cannot be force-updated.

What was done instead:

- **Additive change**: responses expose `price` (euros, always two decimals, e.g. `120.50`) **next to** `price_cents`.
- **Cents stay the source of truth** in the database and the domain. `price` is formatted from integer cents and never goes through `float64`.
- **Input accepts either** `price` or `price_cents` during the migration. Sending both is rejected (`400`). Sub-cent or negative prices are rejected.
- `price_cents` is marked **deprecated** in the OpenAPI spec. It will be removed once web and mobile clients have migrated, measured from usage, with a sunset date agreed with those teams.

Recommended long-term format: `{"amount": "120.50", "currency": "EUR"}`. A string amount keeps exact precision in every client, JavaScript included, and supports other currencies.


### Bulk import: atomic and idempotent

The import inserted ads one by one without a transaction: a failure in the middle left the first ads in the database, and retrying the request created them again.

- **Atomic**: the whole import runs in one transaction. It creates every ad or none.
- **Idempotent**: with an `Idempotency-Key` header, the key is stored in `ad_imports` in the same transaction as the ads. A retry with the same key returns the original ids (`Idempotent-Replayed: true`) without creating anything. This covers the case where the import committed but the response was lost. A failed import rolls the key back too, so it can be retried with the same key.
- **Concurrency**: the key is claimed with `INSERT ... ON CONFLICT DO NOTHING` on the primary key, so a concurrent import with the same key waits for the first one and then replays its result.
- **Safety**: a key reused with a different payload is rejected (`422`, detected with a SHA-256 of the payload). Imports are capped at 1000 ads (`413`) to keep transactions short.
- The header is optional so existing clients keep working; making it mandatory can be agreed with client teams later.
- A unique constraint on the ad content was rejected: a seller may legitimately publish identical ads.
- Follow-ups: purge old keys (e.g. after 24 hours) with a scheduled job; use a multi-row `INSERT` or `COPY` if imports become frequent.



### Publication quota: 5 ads per rolling 24 hours

- **Rolling window**: publications are counted over the last 24 hours (`published_at > now() - 24h`), not per calendar day, so the quota cannot be doubled around midnight.
- **Concurrency-safe**: the check and the insert run in one transaction, behind a per-user PostgreSQL advisory lock (`pg_advisory_xact_lock`). Concurrent requests from the same user are serialized; other users are not affected. A Go mutex would only protect a single instance of the API.
- **Publications log**: the quota counts rows of `ad_publications`, written in the same transaction as the ad. Deleting an ad does not give a slot back, and bulk imports never write there, so they are excluded by construction.
- **Client-friendly**: an exceeded quota answers `429 Too Many Requests` with a `Retry-After` header (seconds until the oldest publication leaves the window).
- **Interpretation to confirm with the product owner**: "it does not apply to the bulk import" was read as "imports are not limited and do not consume the POST /ads quota".
- Follow-ups: purge publications older than the window with a scheduled job; make the limit configurable if product needs it.

### Production readiness

The instructions ask for code "you feel comfortable shipping in production". Beyond the six items, the following gaps were closed.

#### Input validation

- **Domain rules**: an ad needs a non-blank title (200 characters max), a price between 0 and 100 million euros, and an absolute `http(s)` photo URL (2048 characters max). Zero is allowed: free items are common on a classifieds site.
- **One place for the rules**: they live in the `domain` package and apply to `POST /ads` and to the bulk import alike; the HTTP layer only checks the format (valid JSON, readable price).
- **Checked before any database call**: an invalid ad opens no transaction, takes no lock and consumes no quota. A single invalid ad rejects the whole import up front, with its position in the message (`ad #1: invalid ad: ...`).
- **Security**: restricting photo URLs to `http(s)` blocks `javascript:` URLs, which web and mobile clients would otherwise render (XSS).
- **Explicit errors**: `400` answers carry a JSON body (`{"error": "..."}`) that clients can show, instead of an empty response.

#### Read performance (`GET /ads`)

- **Pagination**: the endpoint returned the whole table. It now returns one page (`limit`, 20 by default, 100 max; `offset`, 10,000 max). `has_more` is computed by fetching one extra row, which avoids a `COUNT(*)` over the whole table.
- **Stable order**: `ORDER BY created_at DESC, id DESC`. Without an `ORDER BY`, PostgreSQL returns rows in any order, so pages could overlap or skip ads.
- **Indexes** (migration `005`): a trigram index (`pg_trgm`) for the title search, the only kind of index that serves `ILIKE '%bike%'`, plus b-tree indexes for the default listing, the ads of a seller and price ranges.
- **Index-friendly query**: `user_id::text = $1` cast the column and defeated its index; `user_id` is now compared as a UUID, and the parameter is validated (`400` instead of a database error).
- **Literal search**: `%` and `_` in the title filter are escaped, so searching `100%` no longer matches every ad.
- **Connection pool**: configurable (`DB_MAX_OPEN_CONNS`, 20 by default) and fully kept idle, so read peaks do not reopen connections.
- Measured on 200,000 ads: listing 0.08 ms (it returned every row before), title search 87 ms down to 4 ms, ads of a seller 26 ms down to 0.15 ms.

#### HTTP server

- **Body size limit**: 4 MB (`413`). A full import of 1000 ads stays well below it; without a limit, one large request could exhaust the memory.
- **Timeouts**: Go sets none by default. Reading the request, writing the response and idle keep-alive connections are now bounded, so slow clients cannot hold connections forever.
- **Panic recovery**: a middleware turns a panic into a `500` and logs the stack trace; `net/http` alone drops the connection without answering.
- **Fail fast on wiring**: with manual dependency injection, a usecase left out of `main.go` compiled and panicked on its first request (it happened twice while working on this). `NewRouter` now refuses to start, and a test checks that every route answers.
- **Strict JSON**: unknown fields and trailing data after the JSON value are rejected with a message.

#### Image and CI

- **Multi-stage image**: the binary is built in `golang` and shipped alone in `alpine` (about 20 MB instead of 300 MB, no compiler or sources in production), run by an unprivileged user. Alpine rather than distroless because the Compose healthcheck uses `wget`.
- **CI** (GitHub Actions) on every push and pull request: `gofmt`, `go vet`, tests with the race detector, `govulncheck` for known vulnerabilities, and the image build.

#### Follow-ups

- Keyset pagination (`WHERE (created_at, id) < cursor`) to keep deep pages cheap; it changes the API contract, so it should be agreed with the web and mobile teams.
- `CREATE INDEX CONCURRENTLY` and a migration tool (golang-migrate, goose) in production.
- Observability: access logs, metrics and tracing (OpenTelemetry), and a readiness endpoint that checks the database.
- Real authentication (JWT): the `Authorization` header is trusted as-is, so a malformed user id still reaches the database.
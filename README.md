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

| Method | Path          | Auth | Description                                                                       |
|--------|---------------|------|-----------------------------------------------------------------------------------|
| `POST` | `/ads`        | yes | Create an ad                                                                      |
| `GET` | `/ads/{id}`   | no | Get a single ad                                                                   |
| `GET` | `/ads`        | no | List ads (filterable by `title`, `user_id`, `min_price_cents`, `max_price_cents`) |
| `POST` | `/ads/import` | yes | Bulk ad import                                                                    |
| `DELETE` | `/ads/{id}`   | yes | Delete an ad (owner only)                                                         |
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

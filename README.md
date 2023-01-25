# Equipment Maintenance API

Equipment Maintenance API is a Go service for managing industrial equipment and its maintenance lifecycle. The current foundation exposes a health endpoint, validates all startup configuration, and shuts down gracefully. Later phases add PostgreSQL persistence, authentication, equipment, and work-order workflows.

> Technology baseline date: January 10, 2023. This project intentionally uses the versions of Go and libraries available as of that date.

## Requirements

- Go 1.19
- environment variables listed below

## Run locally

Copy the example configuration, replace the placeholder JWT secret, and export the variables before starting the service:

```sh
cp .env.example .env
set -a
. ./.env
set +a
make run
```

The application reads the process environment directly; it does not load `.env` files itself. A successful health check returns HTTP 200:

```sh
curl -i http://localhost:8080/health
```

```json
{"status":"ok"}
```

## Configuration

| Variable | Description | Example |
| --- | --- | --- |
| `HTTP_ADDRESS` | HTTP listen address in host:port form | `:8080` |
| `DATABASE_URL` | PostgreSQL connection URL | `postgres://equipment:equipment@localhost:5432/equipment?sslmode=disable` |
| `JWT_SECRET` | Secret used to sign access tokens | a long random value |
| `JWT_TTL` | Positive Go duration for access-token lifetime | `15m` |
| `DB_MAX_CONNECTIONS` | Maximum database pool size, at least 1 | `10` |
| `DB_MIN_CONNECTIONS` | Minimum database pool size, from 0 through the maximum | `2` |

Startup fails with a combined, actionable error when configuration is missing or invalid. Database settings are validated now and will be consumed when persistence is introduced.

## Development commands

```sh
make fmt       # format Go source
make test      # run tests
make test-race # run tests with the race detector
make vet       # run static analysis
make check     # run all verification commands
```

## Architecture

The repository uses a domain-oriented layout. `cmd/api` is the executable composition root. Packages under `internal` separate configuration and HTTP infrastructure from the `user`, `equipment`, `workorder`, and `maintenance` domains. Handlers own HTTP concerns, services will own business rules, and repositories will own parameterized SQL.

Only `GET /health` is implemented in this phase. See `docs/SPEC.md` for the complete product requirements and `docs/PLAN.md` for delivery status.

# Equipment Maintenance API

Equipment Maintenance API is a Go service for managing industrial equipment and its maintenance lifecycle. The current application provides user registration, JWT authentication, administrator role management, a health endpoint, PostgreSQL persistence through `pgxpool`, and graceful shutdown. Later phases add equipment and work-order workflows.

## Requirements

- Go 1.19
- Docker with Docker Compose
- environment variables listed below

The database foundation uses PostgreSQL 14.6, `pgx`/`pgxpool` 4.17.2, and `golang-migrate` 4.15.2. All were available before the technology baseline date.

## Run locally

Copy the example configuration, replace the placeholder JWT secret, start PostgreSQL, and apply migrations before starting the service:

```sh
cp .env.example .env
set -a
. ./.env
set +a
make db-up
make db-wait
make migrate-up
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

Startup fails with a combined, actionable error when configuration is missing or invalid. The service also verifies its database connection at startup and exits clearly if PostgreSQL is unavailable.

## PostgreSQL and migrations

Docker Compose runs PostgreSQL 14.6 only; the Go application runs directly on the host. The database uses a named volume so `make db-down` preserves local data.

```sh
make db-up          # start PostgreSQL 14.6
make db-wait        # wait until PostgreSQL accepts connections
make migrate-up     # apply all pending migrations
make migrate-version
make migrate-down   # roll back one migration
make db-down        # stop PostgreSQL and preserve data
make db-clean       # stop PostgreSQL and remove local database data
```

Migration files live in `migrations/` as matching, sequentially numbered `.up.sql` and `.down.sql` files. Migrations are explicit SQL and each multi-statement migration is transaction-wrapped. Apply migrations explicitly before starting a newly deployed application version; the API does not mutate its schema automatically.

The initial migration creates `users` with normalized unique email, role and non-empty value constraints, timezone-aware timestamps, and a role index. Rollback drops the table.

## Users and authentication

Public registration never accepts a role. The first committed registration becomes `admin`; PostgreSQL serializes the empty-table decision so concurrent requests cannot create multiple initial administrators. Every later registration receives `viewer`.

Passwords must contain 8 to 72 bytes. Emails are trimmed and lowercased before lookup and storage, while passwords are stored only as bcrypt hashes. Login returns an HS256 JWT using `JWT_SECRET` and `JWT_TTL`. Password hashes and the signing secret are excluded from API responses.

```sh
curl -X POST http://localhost:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"change-me-now","full_name":"Initial Admin"}'

curl -X POST http://localhost:8080/api/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"admin@example.com","password":"change-me-now"}'

curl http://localhost:8080/api/v1/users/me \
  -H 'Authorization: Bearer <access_token>'
```

Authentication errors use the same JSON envelope as other API errors:

```json
{"error":{"code":"invalid_credentials","message":"Email or password is incorrect"}}
```

Administrators can list users, fetch a user by id, and change another user's role. Tokens from non-admin users are rejected with HTTP 403 at the route layer, and the user service checks the current database role again before doing the work. The last administrator cannot be demoted.

```sh
curl http://localhost:8080/api/v1/admin/users \
  -H 'Authorization: Bearer <admin_access_token>'

curl http://localhost:8080/api/v1/admin/users/2 \
  -H 'Authorization: Bearer <admin_access_token>'

curl -X PATCH http://localhost:8080/api/v1/admin/users/2/role \
  -H 'Authorization: Bearer <admin_access_token>' \
  -H 'Content-Type: application/json' \
  -d '{"role":"dispatcher"}'
```

## Development commands

```sh
make fmt       # format Go source
make test      # run tests
make test-race # run tests with the race detector
make test-integration # start PostgreSQL and test pool/migrations
make vet       # run static analysis
make check     # run all verification commands
```

`make test-integration` creates a disposable PostgreSQL instance under a separate Compose project on port `55432`, validates `up → down → up`, and removes its container and volume afterward. Override `TEST_POSTGRES_PORT` if that port is already in use. Never point the integration test at a database containing data that must be preserved because it deliberately exercises rollback.

## Architecture

The repository uses a domain-oriented layout. `cmd/api` is the executable composition root and `cmd/migrate` is the migration runner. Packages under `internal` separate configuration, database, and HTTP infrastructure from the `user`, `equipment`, `workorder`, and `maintenance` domains. Handlers own HTTP concerns, services will own business rules, and repositories will own parameterized SQL.

The implemented HTTP surface is `GET /health`, `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, authenticated `GET /api/v1/users/me`, and administrator-only `GET /api/v1/admin/users`, `GET /api/v1/admin/users/{id}`, and `PATCH /api/v1/admin/users/{id}/role`. Equipment and work-order endpoints are added in subsequent phases.

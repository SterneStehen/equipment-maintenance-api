# Equipment Maintenance API

Equipment Maintenance API is a Go service for managing industrial equipment and its maintenance lifecycle. The current application provides user registration, JWT authentication, administrator role management, equipment CRUD-style workflows, work-order creation/querying, a health endpoint, PostgreSQL persistence through `pgxpool`, and graceful shutdown.

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

The application reads the process environment directly; it does not load `.env` files itself. A successful liveness check returns HTTP 200:

```sh
curl -i http://localhost:8080/health
```

```json
{"status":"ok"}
```

Readiness also pings PostgreSQL:

```sh
curl -i http://localhost:8080/ready
```

```json
{"status":"ready"}
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

The first migration creates `users` with normalized unique email, role and non-empty value constraints, timezone-aware timestamps, and a role index. The second migration creates `equipment` with normalized unique serial numbers, status constraints, timestamps, and decommission tracking. The third migration creates `work_orders` with equipment/user references, status and priority constraints, assignee tracking, and completion timestamps. The fourth migration stores work-order transition history. The fifth adds work-order comments and one maintenance record per completed work order. Rollback drops the matching table for each migration.

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

## Equipment

Authenticated users can read equipment records. Administrators and dispatchers can create and update equipment. Only administrators can decommission equipment. The API does not physically delete equipment records; `DELETE /api/v1/equipment/{id}` returns HTTP 405 and clients should use the decommission endpoint instead.

Serial numbers are trimmed, uppercased, and unique. Duplicate serial numbers return HTTP 409. Update accepts `active` and `maintenance` statuses; `decommissioned` is only set through the decommission operation.

```sh
curl -X POST http://localhost:8080/api/v1/equipment \
  -H 'Authorization: Bearer <admin_or_dispatcher_token>' \
  -H 'Content-Type: application/json' \
  -d '{"serial_number":"pump-100","name":"Main pump","model":"MX","location":"Line A"}'

curl 'http://localhost:8080/api/v1/equipment?status=active&q=pump&limit=20&offset=0' \
  -H 'Authorization: Bearer <access_token>'

curl -X PATCH http://localhost:8080/api/v1/equipment/1 \
  -H 'Authorization: Bearer <admin_or_dispatcher_token>' \
  -H 'Content-Type: application/json' \
  -d '{"name":"Main pump","model":"MX2","location":"Line A","status":"maintenance","notes":"bearing check"}'

curl -X POST http://localhost:8080/api/v1/equipment/1/decommission \
  -H 'Authorization: Bearer <admin_access_token>'
```

## Work orders

Authenticated users can list and read work orders. Administrators and dispatchers can create and update them. A work order cannot be opened or updated for decommissioned equipment. `assigned_to` is optional, but when supplied it must point to an existing user with the `technician` role.

Work-order state changes use explicit transition endpoints. Allowed transitions are `open -> in_progress`, `open -> canceled`, `in_progress -> completed`, `in_progress -> canceled`, and `completed -> closed`. Closed and canceled work orders are terminal. Every transition writes a history row in the same database transaction. Completing a work order also creates its maintenance record in that same transaction, so a partial completion is rolled back. Assigned technicians may start and complete only their own work orders; administrators and dispatchers may run operational transitions.

Supported filters are `status`, `priority`, `equipment_id`, `assigned_to`, `q`, `limit`, and `offset`. Authenticated users can add and list comments on a work order.

```sh
curl -X POST http://localhost:8080/api/v1/work-orders \
  -H 'Authorization: Bearer <admin_or_dispatcher_token>' \
  -H 'Content-Type: application/json' \
  -d '{"equipment_id":1,"title":"Replace belt","priority":"high","assigned_to":3}'

curl 'http://localhost:8080/api/v1/work-orders?status=open&priority=high&equipment_id=1&limit=20&offset=0' \
  -H 'Authorization: Bearer <access_token>'

curl -X PATCH http://localhost:8080/api/v1/work-orders/1 \
  -H 'Authorization: Bearer <admin_or_dispatcher_token>' \
  -H 'Content-Type: application/json' \
  -d '{"title":"Replace belt","priority":"urgent","assigned_to":3}'

curl -X POST http://localhost:8080/api/v1/work-orders/1/start \
  -H 'Authorization: Bearer <assigned_technician_token>' \
  -H 'Content-Type: application/json' \
  -d '{"note":"starting now"}'

curl -X POST http://localhost:8080/api/v1/work-orders/1/complete \
  -H 'Authorization: Bearer <assigned_technician_token>'

curl -X POST http://localhost:8080/api/v1/work-orders/1/close \
  -H 'Authorization: Bearer <admin_or_dispatcher_token>'

curl -X POST http://localhost:8080/api/v1/work-orders/1/comments \
  -H 'Authorization: Bearer <access_token>' \
  -H 'Content-Type: application/json' \
  -d '{"body":"left a note for next shift"}'

curl 'http://localhost:8080/api/v1/work-orders/1/comments?limit=20&offset=0' \
  -H 'Authorization: Bearer <access_token>'
```

## Maintenance records

Maintenance records are created when a work order is completed. Authenticated users can list them and filter by `work_order_id`, `equipment_id`, `performed_by`, `limit`, and `offset`. If the completion flow cannot write the maintenance record, the work-order status and history insert are rolled back together.

```sh
curl 'http://localhost:8080/api/v1/maintenance-records?equipment_id=1&limit=20&offset=0' \
  -H 'Authorization: Bearer <access_token>'
```

## Development commands

```sh
make fmt       # format Go source
make test      # run tests
make test-race # run tests with the race detector
make test-integration # start PostgreSQL and test pool/migrations
make vet       # run static analysis
make check     # run all verification commands
make release-check # run local and clean-db integration checks
```

`make test-integration` creates a disposable PostgreSQL instance under a separate Compose project on port `55432`, validates `up → down → up`, and removes its container and volume afterward. Override `TEST_POSTGRES_PORT` if that port is already in use. Never point the integration test at a database containing data that must be preserved because it deliberately exercises rollback.

## API specification

The repository includes a hand-maintained OpenAPI 3.0 spec in `openapi.yaml`.

## Architecture

The repository uses a domain-oriented layout. `cmd/api` is the executable composition root and `cmd/migrate` is the migration runner. Packages under `internal` separate configuration, database, and HTTP infrastructure from the `user`, `equipment`, `workorder`, and `maintenance` domains. Handlers own HTTP concerns, services will own business rules, and repositories will own parameterized SQL.

The implemented HTTP surface is `GET /health`, `GET /ready`, auth/user endpoints, administrator role endpoints, authenticated equipment endpoints under `/api/v1/equipment`, authenticated work-order endpoints under `/api/v1/work-orders`, and authenticated maintenance-record listing under `/api/v1/maintenance-records`.

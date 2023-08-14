# Release readiness audit

Audit window: August 2023.

## Scope

- matched implementation against the phase requirements from configuration through maintenance records
- checked authn/authz code paths, transaction boundaries, SQL filters, pagination defaults and limits
- checked migration lifecycle, including a rollback case with closed work orders
- scanned tracked files and commit history for likely secrets or generated junk
- checked Go 1.19 compatibility and dependency vintage

## Finding fixed

Closed work orders were losing `completed_at`, and rolling migration 000004 down over existing closed rows could violate the `work_orders_completed_time` constraint. The migration and transition update now keep completion time for `closed` work orders, and the rollback path converts closed rows back to completed rows with a timestamp.

## Secret/artifact scan notes

The scan found only placeholder values and test fixtures:

- `.env.example` contains `replace-with-a-long-random-secret`
- tests use short fake JWT secrets and fake passwords
- commit history contains words like `password`, `token`, and `secret`, but no private keys or provider tokens

No `.env`, key, certificate, database, log, binary test artifact, or generated file is tracked.

## Go and dependency date notes

Checked with Go 1.19.13. `go.mod` declares `go 1.19`.

Direct dependencies checked through `go list -m -json <module>@<version>`:

| module | version | module time | go version |
| --- | --- | --- | --- |
| `github.com/gin-gonic/gin` | `v1.8.2` | 2022-12-22 | 1.18 |
| `github.com/golang-jwt/jwt/v4` | `v4.4.3` | 2022-11-08 | 1.16 |
| `github.com/golang-migrate/migrate/v4` | `v4.15.2` | 2022-03-17 | 1.16 |
| `github.com/jackc/pgconn` | `v1.13.0` | 2022-08-06 | 1.12 |
| `github.com/jackc/pgx/v4` | `v4.17.2` | 2022-09-03 | 1.13 |
| `github.com/stretchr/testify` | `v1.8.1` | 2022-10-20 | 1.13 |
| `golang.org/x/crypto` | `v0.0.0-20220722155217-630584e8d5aa` | 2022-07-22 | 1.17 |

No direct dependency requires Go newer than 1.19.

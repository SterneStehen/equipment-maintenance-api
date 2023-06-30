POSTGRES_DB ?= equipment
POSTGRES_USER ?= equipment
POSTGRES_PASSWORD ?= equipment
POSTGRES_PORT ?= 5432
DATABASE_URL ?= postgres://equipment:equipment@localhost:5432/equipment?sslmode=disable
TEST_POSTGRES_PORT ?= 55432
TEST_COMPOSE_PROJECT ?= equipment-maintenance-api-test
COMPOSE_ENV = POSTGRES_DB="$(POSTGRES_DB)" POSTGRES_USER="$(POSTGRES_USER)" POSTGRES_PASSWORD="$(POSTGRES_PASSWORD)" POSTGRES_PORT="$(POSTGRES_PORT)"

.PHONY: run fmt test test-race test-integration vet check \
	db-up db-wait db-down db-clean migrate-up migrate-down migrate-version

run:
	go run ./cmd/api

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

test:
	go test ./...

test-race:
	go test -race ./...

test-integration:
	@set -eu; \
	cleanup() { $(COMPOSE_ENV) POSTGRES_PORT="$(TEST_POSTGRES_PORT)" docker compose -p "$(TEST_COMPOSE_PROJECT)" down --volumes --remove-orphans; }; \
	trap cleanup EXIT INT TERM; \
	cleanup; \
	$(COMPOSE_ENV) POSTGRES_PORT="$(TEST_POSTGRES_PORT)" docker compose -p "$(TEST_COMPOSE_PROJECT)" up -d postgres; \
	ready=false; \
	for attempt in $$(seq 1 30); do \
		if docker compose -p "$(TEST_COMPOSE_PROJECT)" exec -T postgres pg_isready -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)" >/dev/null 2>&1; then ready=true; break; fi; \
		sleep 1; \
	done; \
	if [ "$$ready" != true ]; then docker compose -p "$(TEST_COMPOSE_PROJECT)" logs postgres; exit 1; fi; \
	TEST_DATABASE_URL="postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@127.0.0.1:$(TEST_POSTGRES_PORT)/$(POSTGRES_DB)?sslmode=disable" go test -count=1 -p=1 -tags=integration ./internal/database ./internal/user ./internal/equipment ./internal/workorder ./internal/maintenance

vet:
	go vet ./...

check: fmt test test-race vet

db-up:
	$(COMPOSE_ENV) docker compose up -d postgres

db-wait:
	@ready=false; \
	for attempt in $$(seq 1 30); do \
		if docker compose exec -T postgres pg_isready -U "$(POSTGRES_USER)" -d "$(POSTGRES_DB)"; then ready=true; break; fi; \
		sleep 1; \
	done; \
	if [ "$$ready" != true ]; then docker compose logs postgres; exit 1; fi

db-down:
	$(COMPOSE_ENV) docker compose down

db-clean:
	$(COMPOSE_ENV) docker compose down --volumes --remove-orphans

migrate-up:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate up

migrate-down:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate down 1

migrate-version:
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate version

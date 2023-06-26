//go:build integration
// +build integration

package database

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

func TestPostgreSQLPoolAndMigrationLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	pool, err := Open(ctx, Config{
		URL:            databaseURL,
		MaxConnections: 4,
		MinConnections: 1,
	})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer pool.Close()

	var pgMajor int
	if err := pool.QueryRow(ctx, "SELECT current_setting('server_version_num')::integer / 10000").Scan(&pgMajor); err != nil {
		t.Fatalf("query PostgreSQL version: %v", err)
	}
	if pgMajor != 14 {
		t.Fatalf("PostgreSQL major version = %d, want 14", pgMajor)
	}

	migrator := openTestMigrator(t, databaseURL)
	defer closeTestMigrator(t, migrator)
	if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("reset migrations: %v", err)
	}

	if err := migrator.Up(); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	assertMigrationVersion(t, migrator, 5)
	assertUsersConstraints(t, ctx, pool)
	assertEquipmentConstraints(t, ctx, pool)
	assertWorkOrderConstraints(t, ctx, pool)
	assertWorkOrderHistoryConstraints(t, ctx, pool)
	assertCommentConstraints(t, ctx, pool)
	assertMaintenanceRecordConstraints(t, ctx, pool)

	if err := migrator.Down(); err != nil {
		t.Fatalf("roll back migrations: %v", err)
	}
	assertUsersTableMissing(t, ctx, pool)
	assertEquipmentTableMissing(t, ctx, pool)
	assertWorkOrdersTableMissing(t, ctx, pool)
	assertWorkOrderHistoryTableMissing(t, ctx, pool)
	assertWorkOrderCommentsTableMissing(t, ctx, pool)
	assertMaintenanceRecordsTableMissing(t, ctx, pool)

	if err := migrator.Up(); err != nil {
		t.Fatalf("reapply migrations: %v", err)
	}
	assertMigrationVersion(t, migrator, 5)
}

func openTestMigrator(t *testing.T, databaseURL string) *migrate.Migrate {
	t.Helper()
	migrator, err := NewMigrator(databaseURL, "../../migrations")
	if err != nil {
		t.Fatalf("NewMigrator() error = %v", err)
	}
	return migrator
}

func closeTestMigrator(t *testing.T, migrator *migrate.Migrate) {
	t.Helper()
	sourceErr, databaseErr := migrator.Close()
	if sourceErr != nil || databaseErr != nil {
		t.Errorf("close migrator: source error = %v, database error = %v", sourceErr, databaseErr)
	}
}

func assertMigrationVersion(t *testing.T, migrator *migrate.Migrate, want uint) {
	t.Helper()
	version, dirty, err := migrator.Version()
	if err != nil {
		t.Fatalf("read migration version: %v", err)
	}
	if version != want || dirty {
		t.Fatalf("migration version = %d (dirty=%t), want %d (dirty=false)", version, dirty, want)
	}
}

func assertUsersConstraints(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var cols int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'users'
		  AND column_name IN ('id', 'email', 'password_hash', 'full_name', 'role', 'created_at', 'updated_at')
	`).Scan(&cols); err != nil {
		t.Fatalf("inspect users columns: %v", err)
	}
	if cols != 7 {
		t.Fatalf("users expected column count = %d, want 7", cols)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('admin@example.com', 'hash', 'Initial Administrator', 'admin')
	`); err != nil {
		t.Fatalf("insert valid user: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('admin@example.com', 'hash', 'Duplicate Administrator', 'viewer')
	`); err == nil {
		t.Fatal("duplicate normalized email was accepted")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('other@example.com', 'hash', 'Invalid Role', 'owner')
	`); err == nil {
		t.Fatal("invalid role was accepted")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('Not-Normalized@example.com', 'hash', 'Invalid Email', 'viewer')
	`); err == nil {
		t.Fatal("non-normalized email was accepted")
	}
}

func assertEquipmentConstraints(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var cols int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'equipment'
		  AND column_name IN ('id', 'serial_number', 'name', 'model', 'location', 'status', 'notes', 'created_at', 'updated_at', 'decommissioned_at')
	`).Scan(&cols); err != nil {
		t.Fatalf("inspect equipment columns: %v", err)
	}
	if cols != 10 {
		t.Fatalf("equipment expected column count = %d, want 10", cols)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO equipment (serial_number, name, status)
		VALUES ('PUMP-100', 'Pump 100', 'active')
	`); err != nil {
		t.Fatalf("insert valid equipment: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO equipment (serial_number, name, status)
		VALUES ('PUMP-100', 'Duplicate Pump', 'active')
	`); err == nil {
		t.Fatal("duplicate serial number was accepted")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO equipment (serial_number, name, status)
		VALUES ('not-normalized', 'Bad Serial', 'active')
	`); err == nil {
		t.Fatal("non-normalized serial number was accepted")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO equipment (serial_number, name, status)
		VALUES ('PUMP-101', 'Bad Status', 'gone')
	`); err == nil {
		t.Fatal("invalid equipment status was accepted")
	}
}

func assertWorkOrderConstraints(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var cols int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'work_orders'
		  AND column_name IN ('id', 'equipment_id', 'title', 'description', 'status', 'priority', 'assigned_to', 'created_by', 'created_at', 'updated_at', 'completed_at')
	`).Scan(&cols); err != nil {
		t.Fatalf("inspect work_orders columns: %v", err)
	}
	if cols != 11 {
		t.Fatalf("work_orders expected column count = %d, want 11", cols)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO work_orders (equipment_id, title, status, priority, created_by)
		VALUES (1, 'Fix pump', 'open', 'high', 1)
	`); err != nil {
		t.Fatalf("insert valid work order: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO work_orders (equipment_id, title, status, priority, created_by)
		VALUES (1, '', 'open', 'high', 1)
	`); err == nil {
		t.Fatal("blank work order title was accepted")
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO work_orders (equipment_id, title, status, priority, created_by)
		VALUES (1, 'Bad status', 'waiting', 'high', 1)
	`); err == nil {
		t.Fatal("invalid work order status was accepted")
	}
}

func assertWorkOrderHistoryConstraints(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var cols int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'work_order_history'
		  AND column_name IN ('id', 'work_order_id', 'from_status', 'to_status', 'actor_id', 'note', 'created_at')
	`).Scan(&cols); err != nil {
		t.Fatalf("inspect work_order_history columns: %v", err)
	}
	if cols != 7 {
		t.Fatalf("work_order_history expected column count = %d, want 7", cols)
	}
}

func assertCommentConstraints(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var cols int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'work_order_comments'
		  AND column_name IN ('id', 'work_order_id', 'author_id', 'body', 'created_at')
	`).Scan(&cols); err != nil {
		t.Fatalf("inspect work_order_comments columns: %v", err)
	}
	if cols != 5 {
		t.Fatalf("work_order_comments expected column count = %d, want 5", cols)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO work_order_comments (work_order_id, author_id, body)
		VALUES (1, 1, 'looks done')
	`); err != nil {
		t.Fatalf("insert valid work order comment: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO work_order_comments (work_order_id, author_id, body)
		VALUES (1, 1, '')
	`); err == nil {
		t.Fatal("blank work order comment was accepted")
	}
}

func assertMaintenanceRecordConstraints(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var cols int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND table_name = 'maintenance_records'
		  AND column_name IN ('id', 'work_order_id', 'equipment_id', 'performed_by', 'notes', 'created_at')
	`).Scan(&cols); err != nil {
		t.Fatalf("inspect maintenance_records columns: %v", err)
	}
	if cols != 6 {
		t.Fatalf("maintenance_records expected column count = %d, want 6", cols)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO maintenance_records (work_order_id, equipment_id, performed_by, notes)
		VALUES (1, 1, 1, 'belt changed')
	`); err != nil {
		t.Fatalf("insert valid maintenance record: %v", err)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO maintenance_records (work_order_id, equipment_id, performed_by, notes)
		VALUES (1, 1, 1, 'duplicate')
	`); err == nil {
		t.Fatal("duplicate maintenance record for work order was accepted")
	}
}

func assertUsersTableMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var tableName *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.users')::text").Scan(&tableName); err != nil {
		t.Fatalf("check users table after rollback: %v", err)
	}
	if tableName != nil {
		t.Fatalf("users table still exists after rollback: %s", *tableName)
	}
}

func assertEquipmentTableMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var tableName *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.equipment')::text").Scan(&tableName); err != nil {
		t.Fatalf("check equipment table after rollback: %v", err)
	}
	if tableName != nil {
		t.Fatalf("equipment table still exists after rollback: %s", *tableName)
	}
}

func assertWorkOrdersTableMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var tableName *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.work_orders')::text").Scan(&tableName); err != nil {
		t.Fatalf("check work_orders table after rollback: %v", err)
	}
	if tableName != nil {
		t.Fatalf("work_orders table still exists after rollback: %s", *tableName)
	}
}

func assertWorkOrderHistoryTableMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var tableName *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.work_order_history')::text").Scan(&tableName); err != nil {
		t.Fatalf("check work_order_history table after rollback: %v", err)
	}
	if tableName != nil {
		t.Fatalf("work_order_history table still exists after rollback: %s", *tableName)
	}
}

func assertWorkOrderCommentsTableMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var tableName *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.work_order_comments')::text").Scan(&tableName); err != nil {
		t.Fatalf("check work_order_comments table after rollback: %v", err)
	}
	if tableName != nil {
		t.Fatalf("work_order_comments table still exists after rollback: %s", *tableName)
	}
}

func assertMaintenanceRecordsTableMissing(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	var tableName *string
	if err := pool.QueryRow(ctx, "SELECT to_regclass('public.maintenance_records')::text").Scan(&tableName); err != nil {
		t.Fatalf("check maintenance_records table after rollback: %v", err)
	}
	if tableName != nil {
		t.Fatalf("maintenance_records table still exists after rollback: %s", *tableName)
	}
}

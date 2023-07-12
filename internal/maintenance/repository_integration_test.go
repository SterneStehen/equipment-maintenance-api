//go:build integration
// +build integration

package maintenance

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	appdb "github.com/SterneStehen/equipment-maintenance-api/internal/database"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMaintenanceRepositoryList(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	m, err := appdb.NewMigrator(dbURL, "../../migrations")
	require.NoError(t, err)
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("apply migrations: %v", err)
	}
	srcErr, dbErr := m.Close()
	require.NoError(t, srcErr)
	require.NoError(t, dbErr)

	pool, err := appdb.Open(ctx, appdb.Config{URL: dbURL, MaxConnections: 5, MinConnections: 1})
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, "TRUNCATE maintenance_records, work_order_comments, work_order_history, work_orders, equipment, users RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	tech, eqID := seedMaintenanceRows(t, ctx, pool)

	repo := NewRepository(pool)
	arr, err := repo.List(ctx, ListFilter{EquipmentID: eqID, PerformedBy: tech, Limit: 10})
	require.NoError(t, err)
	require.Len(t, arr, 2)
	assert.Equal(t, "second", arr[0].Notes)

	page, err := repo.List(ctx, ListFilter{EquipmentID: eqID, Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, "first", page[0].Notes)
}

func seedMaintenanceRows(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (int64, int64) {
	t.Helper()
	var dispatcher, tech, eqID, firstWO, secondWO int64
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('maint-dispatcher@example.com', 'hash', 'Dispatcher', 'dispatcher')
		RETURNING id
	`).Scan(&dispatcher))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('maint-tech@example.com', 'hash', 'Technician', 'technician')
		RETURNING id
	`).Scan(&tech))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO equipment (serial_number, name, status)
		VALUES ('MAINT-PUMP-1', 'Maintenance Pump', 'active')
		RETURNING id
	`).Scan(&eqID))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO work_orders (equipment_id, title, status, priority, assigned_to, created_by, completed_at)
		VALUES ($1, 'First', 'completed', 'medium', $2, $3, NOW())
		RETURNING id
	`, eqID, tech, dispatcher).Scan(&firstWO))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO work_orders (equipment_id, title, status, priority, assigned_to, created_by, completed_at)
		VALUES ($1, 'Second', 'completed', 'medium', $2, $3, NOW())
		RETURNING id
	`, eqID, tech, dispatcher).Scan(&secondWO))

	_, err := pool.Exec(ctx, `
		INSERT INTO maintenance_records (work_order_id, equipment_id, performed_by, notes)
		VALUES ($1, $2, $3, 'first'), ($4, $2, $3, 'second')
	`, firstWO, eqID, tech, secondWO)
	require.NoError(t, err)
	return tech, eqID
}

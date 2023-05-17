//go:build integration
// +build integration

package workorder

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	appdb "github.com/SterneStehen/equipment-maintenance-api/internal/database"
	"github.com/SterneStehen/equipment-maintenance-api/internal/equipment"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/golang-migrate/migrate/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkOrderRepositoryFlow(t *testing.T) {
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
	_, err = pool.Exec(ctx, "TRUNCATE work_orders, equipment, users RESTART IDENTITY")
	require.NoError(t, err)

	admin, dispatcher, tech, viewer := seedWOUsers(t, ctx, pool)
	eqRepo := equipment.NewRepository(pool)
	pump, err := equipment.NewService(eqRepo).Create(ctx, user.Actor{Role: user.RoleDispatcher}, equipment.CreateInput{
		SerialNumber: "pump-wo-1", Name: "Pump WO",
	})
	require.NoError(t, err)
	dead, err := equipment.NewService(eqRepo).Create(ctx, user.Actor{Role: user.RoleAdmin}, equipment.CreateInput{
		SerialNumber: "pump-dead-1", Name: "Old Pump",
	})
	require.NoError(t, err)
	_, err = equipment.NewService(eqRepo).Decommission(ctx, user.Actor{Role: user.RoleAdmin}, dead.ID)
	require.NoError(t, err)

	svc := NewService(NewRepository(pool))
	wo, err := svc.Create(ctx, user.Actor{UserID: dispatcher, Role: user.RoleDispatcher}, CreateInput{
		EquipmentID: pump.ID, Title: "Replace belt", Priority: PriorityHigh, AssignedTo: &tech,
	})
	require.NoError(t, err)
	assert.Equal(t, dispatcher, wo.CreatedBy)
	assert.Equal(t, &tech, wo.AssignedTo)

	_, err = svc.Create(ctx, user.Actor{UserID: dispatcher, Role: user.RoleDispatcher}, CreateInput{
		EquipmentID: dead.ID, Title: "Try old pump",
	})
	require.ErrorIs(t, err, ErrEquipmentDecommissioned)

	_, err = svc.Create(ctx, user.Actor{UserID: dispatcher, Role: user.RoleDispatcher}, CreateInput{
		EquipmentID: pump.ID, Title: "Bad assignee", AssignedTo: &viewer,
	})
	require.ErrorIs(t, err, ErrAssigneeNotTechnician)

	updated, err := svc.Update(ctx, user.Actor{UserID: admin, Role: user.RoleAdmin}, wo.ID, UpdateInput{
		Title: "Replace belt", Status: StatusCompleted, Priority: PriorityUrgent, AssignedTo: &tech,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, updated.Status)
	require.NotNil(t, updated.CompletedAt)

	arr, err := svc.List(ctx, user.Actor{Role: user.RoleViewer}, ListFilter{Status: StatusCompleted, AssignedTo: tech, Limit: 10})
	require.NoError(t, err)
	require.Len(t, arr, 1)

	page, err := svc.List(ctx, user.Actor{Role: user.RoleViewer}, ListFilter{Limit: 1, Offset: 0})
	require.NoError(t, err)
	require.Len(t, page, 1)
}

func seedWOUsers(t *testing.T, ctx context.Context, pool *pgxpool.Pool) (int64, int64, int64, int64) {
	t.Helper()
	var admin, dispatcher, tech, viewer int64
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('wo-admin@example.com', 'hash', 'Admin', 'admin')
		RETURNING id
	`).Scan(&admin))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('wo-dispatcher@example.com', 'hash', 'Dispatcher', 'dispatcher')
		RETURNING id
	`).Scan(&dispatcher))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('wo-tech@example.com', 'hash', 'Technician', 'technician')
		RETURNING id
	`).Scan(&tech))
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('wo-viewer@example.com', 'hash', 'Viewer', 'viewer')
		RETURNING id
	`).Scan(&viewer))
	return admin, dispatcher, tech, viewer
}

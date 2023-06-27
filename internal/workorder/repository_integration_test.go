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
	_, err = pool.Exec(ctx, "TRUNCATE maintenance_records, work_order_comments, work_order_history, work_orders, equipment, users RESTART IDENTITY CASCADE")
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

	comment, err := svc.AddComment(ctx, user.Actor{UserID: viewer, Role: user.RoleViewer}, wo.ID, "  watching this ")
	require.NoError(t, err)
	assert.Equal(t, "watching this", comment.Body)

	comments, err := svc.ListComments(ctx, user.Actor{UserID: dispatcher, Role: user.RoleDispatcher}, wo.ID, 10, 0)
	require.NoError(t, err)
	require.Len(t, comments, 1)
	assert.Equal(t, viewer, comments[0].AuthorID)

	_, err = svc.Create(ctx, user.Actor{UserID: dispatcher, Role: user.RoleDispatcher}, CreateInput{
		EquipmentID: dead.ID, Title: "Try old pump",
	})
	require.ErrorIs(t, err, ErrEquipmentDecommissioned)

	_, err = svc.Create(ctx, user.Actor{UserID: dispatcher, Role: user.RoleDispatcher}, CreateInput{
		EquipmentID: pump.ID, Title: "Bad assignee", AssignedTo: &viewer,
	})
	require.ErrorIs(t, err, ErrAssigneeNotTechnician)

	updated, err := svc.Update(ctx, user.Actor{UserID: admin, Role: user.RoleAdmin}, wo.ID, UpdateInput{
		Title: "Replace belt", Status: StatusOpen, Priority: PriorityUrgent, AssignedTo: &tech,
	})
	require.NoError(t, err)
	assert.Equal(t, StatusOpen, updated.Status)

	started, err := svc.Start(ctx, user.Actor{UserID: tech, Role: user.RoleTechnician}, wo.ID, "started")
	require.NoError(t, err)
	assert.Equal(t, StatusInProgress, started.Status)

	_, err = svc.Cancel(ctx, user.Actor{UserID: tech, Role: user.RoleTechnician}, wo.ID, "")
	require.ErrorIs(t, err, ErrPermissionDenied)

	done, err := svc.Complete(ctx, user.Actor{UserID: tech + 20, Role: user.RoleTechnician}, wo.ID, "")
	require.ErrorIs(t, err, ErrTechnicianOwnership)
	assert.Equal(t, WorkOrder{}, done)

	done, err = svc.Complete(ctx, user.Actor{UserID: tech, Role: user.RoleTechnician}, wo.ID, "done")
	require.NoError(t, err)
	assert.Equal(t, StatusCompleted, done.Status)
	require.NotNil(t, done.CompletedAt)

	var recEquipment, recPerformer int64
	var recNotes string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT equipment_id, performed_by, notes
		FROM maintenance_records
		WHERE work_order_id = $1
	`, wo.ID).Scan(&recEquipment, &recPerformer, &recNotes))
	assert.Equal(t, pump.ID, recEquipment)
	assert.Equal(t, tech, recPerformer)
	assert.Equal(t, "done", recNotes)

	closed, err := svc.Close(ctx, user.Actor{UserID: admin, Role: user.RoleAdmin}, wo.ID, "ok")
	require.NoError(t, err)
	assert.Equal(t, StatusClosed, closed.Status)

	_, err = svc.Start(ctx, user.Actor{UserID: admin, Role: user.RoleAdmin}, wo.ID, "")
	require.ErrorIs(t, err, ErrTerminalState)

	arr, err := svc.List(ctx, user.Actor{Role: user.RoleViewer}, ListFilter{Status: StatusClosed, AssignedTo: tech, Limit: 10})
	require.NoError(t, err)
	require.Len(t, arr, 1)

	var histCount int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM work_order_history WHERE work_order_id = $1", wo.ID).Scan(&histCount))
	assert.Equal(t, 3, histCount)

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

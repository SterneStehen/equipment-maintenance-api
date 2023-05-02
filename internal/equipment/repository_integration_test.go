//go:build integration
// +build integration

package equipment

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	appdb "github.com/SterneStehen/equipment-maintenance-api/internal/database"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEquipmentRepositoryFlow(t *testing.T) {
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
	sourceErr, databaseErr := m.Close()
	require.NoError(t, sourceErr)
	require.NoError(t, databaseErr)

	pool, err := appdb.Open(ctx, appdb.Config{URL: dbURL, MaxConnections: 5, MinConnections: 1})
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, "TRUNCATE equipment RESTART IDENTITY")
	require.NoError(t, err)

	svc := NewService(NewRepository(pool))
	admin := user.Actor{UserID: 1, Role: user.RoleAdmin}
	dispatcher := user.Actor{UserID: 2, Role: user.RoleDispatcher}

	first, err := svc.Create(ctx, dispatcher, CreateInput{
		SerialNumber: " pump-1 ", Name: "Main pump", Model: "MX", Location: "Line A", Notes: "new-ish",
	})
	require.NoError(t, err)
	assert.Equal(t, "PUMP-1", first.SerialNumber)
	assert.Equal(t, StatusActive, first.Status)

	_, err = svc.Create(ctx, dispatcher, CreateInput{SerialNumber: "PUMP-1", Name: "Duplicate pump"})
	require.ErrorIs(t, err, ErrSerialTaken)

	second, err := svc.Create(ctx, admin, CreateInput{SerialNumber: "VALVE-9", Name: "Valve", Location: "Line B"})
	require.NoError(t, err)

	updated, err := svc.Update(ctx, dispatcher, first.ID, UpdateInput{
		Name: "Main pump", Model: "MX2", Location: "Line A", Status: StatusMaintenance, Notes: "bearing check",
	})
	require.NoError(t, err)
	assert.Equal(t, StatusMaintenance, updated.Status)
	assert.Equal(t, "MX2", updated.Model)

	arr, err := svc.List(ctx, user.Actor{Role: user.RoleViewer}, ListFilter{Status: StatusMaintenance, Query: "pump", Limit: 10})
	require.NoError(t, err)
	require.Len(t, arr, 1)
	assert.Equal(t, first.ID, arr[0].ID)

	page, err := svc.List(ctx, admin, ListFilter{Limit: 1, Offset: 1})
	require.NoError(t, err)
	require.Len(t, page, 1)
	assert.Equal(t, second.ID, page[0].ID)

	var before int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM equipment").Scan(&before))
	retired, err := svc.Decommission(ctx, admin, first.ID)
	require.NoError(t, err)
	assert.Equal(t, StatusDecommissioned, retired.Status)
	require.NotNil(t, retired.DecommissionedAt)

	var after int
	require.NoError(t, pool.QueryRow(ctx, "SELECT COUNT(*) FROM equipment").Scan(&after))
	assert.Equal(t, before, after)

	_, err = svc.Decommission(ctx, admin, first.ID)
	require.ErrorIs(t, err, ErrAlreadyRetired)
}

//go:build integration
// +build integration

package audit

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	appdb "github.com/SterneStehen/equipment-maintenance-api/internal/database"
	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/require"
)

func TestAuditRepositoryRecord(t *testing.T) {
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
	_, err = pool.Exec(ctx, "TRUNCATE audit_events, users RESTART IDENTITY CASCADE")
	require.NoError(t, err)

	var actorID int64
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ('audit-admin@example.com', 'hash', 'Audit Admin', 'admin')
		RETURNING id
	`).Scan(&actorID))

	repo := NewRepository(pool)
	err = repo.Record(ctx, EventInput{
		ActorID: actorID, Action: "user.role_changed", Target: "user", TargetID: actorID, Details: "role=admin",
	})
	require.NoError(t, err)

	var action, target, details string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT action, target_type, details
		FROM audit_events
		WHERE actor_id = $1
	`, actorID).Scan(&action, &target, &details))
	require.Equal(t, "user.role_changed", action)
	require.Equal(t, "user", target)
	require.Equal(t, "role=admin", details)
}

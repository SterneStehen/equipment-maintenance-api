//go:build integration
// +build integration

package user

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	appdb "github.com/SterneStehen/equipment-maintenance-api/internal/database"
	"github.com/golang-migrate/migrate/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func TestConcurrentInitialRegistrationCreatesOneAdministrator(t *testing.T) {
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

	pool, err := appdb.Open(ctx, appdb.Config{URL: dbURL, MaxConnections: 20, MinConnections: 1})
	require.NoError(t, err)
	defer pool.Close()
	_, err = pool.Exec(ctx, "TRUNCATE users RESTART IDENTITY")
	require.NoError(t, err)

	svc := NewService(NewRepository(pool))
	svc.pass = bcryptPass{cost: bcrypt.MinCost}

	const registrations = 16
	type outcome struct {
		u   User
		err error
	}
	ready := make(chan struct{})
	results := make(chan outcome, registrations)
	var wg sync.WaitGroup

	for i := 0; i < registrations; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-ready
			u, err := svc.Register(ctx, RegisterInput{
				Email: fmt.Sprintf("worker-%02d@example.com", n), Password: "password1", FullName: fmt.Sprintf("Worker %d", n),
			})
			results <- outcome{u: u, err: err}
		}(i)
	}
	close(ready)
	wg.Wait()
	close(results)

	admins := 0
	viewers := 0
	for got := range results {
		require.NoError(t, got.err)
		switch got.u.Role {
		case RoleAdmin:
			admins++
		case RoleViewer:
			viewers++
		default:
			t.Fatalf("registration returned unexpected role %q", got.u.Role)
		}
	}
	assert.Equal(t, 1, admins)
	assert.Equal(t, registrations-1, viewers)

	var dbAdmins, dbViewers int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FILTER (WHERE role = 'admin'), COUNT(*) FILTER (WHERE role = 'viewer')
		FROM users
	`).Scan(&dbAdmins, &dbViewers)
	require.NoError(t, err)
	assert.Equal(t, 1, dbAdmins)
	assert.Equal(t, registrations-1, dbViewers)

	later, err := svc.Register(ctx, RegisterInput{Email: "later@example.com", Password: "password2", FullName: "Later User"})
	require.NoError(t, err)
	assert.Equal(t, RoleViewer, later.Role)

	_, err = svc.Register(ctx, RegisterInput{Email: " LATER@EXAMPLE.COM ", Password: "password3", FullName: "Duplicate"})
	require.ErrorIs(t, err, ErrEmailTaken)

	loggedIn, err := svc.Authenticate(ctx, "later@example.com", "password2")
	require.NoError(t, err)
	assert.Equal(t, later.ID, loggedIn.ID)

	var storedHash string
	require.NoError(t, pool.QueryRow(ctx, "SELECT password_hash FROM users WHERE id = $1", later.ID).Scan(&storedHash))
	assert.NotEqual(t, "password2", storedHash)
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(storedHash), []byte("password2")))
}

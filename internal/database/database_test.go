package database

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestOpenRejectsInvalidPoolSettings(t *testing.T) {
	tests := []struct {
		name        string
		config      Config
		wantInError string
	}{
		{
			name:        "missing URL",
			config:      Config{MaxConnections: 1},
			wantInError: "URL is required",
		},
		{
			name:        "zero maximum",
			config:      Config{URL: "postgres://localhost/equipment", MaxConnections: 0},
			wantInError: "maximum connections",
		},
		{
			name:        "negative minimum",
			config:      Config{URL: "postgres://localhost/equipment", MaxConnections: 1, MinConnections: -1},
			wantInError: "minimum connections must not be negative",
		},
		{
			name:        "minimum exceeds maximum",
			config:      Config{URL: "postgres://localhost/equipment", MaxConnections: 1, MinConnections: 2},
			wantInError: "minimum connections must not exceed maximum",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pool, err := Open(context.Background(), test.config)
			if pool != nil {
				pool.Close()
				t.Fatal("Open() returned a pool for invalid settings")
			}
			if err == nil || !strings.Contains(err.Error(), test.wantInError) {
				t.Fatalf("Open() error = %v, want it to contain %q", err, test.wantInError)
			}
		})
	}
}

func TestOpenReportsConnectionFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	pool, err := Open(ctx, Config{
		URL:            "postgres://equipment:equipment@127.0.0.1:1/equipment?sslmode=disable",
		MaxConnections: 1,
	})
	if pool != nil {
		pool.Close()
		t.Fatal("Open() returned a pool for an unavailable database")
	}
	if err == nil || !strings.Contains(err.Error(), "connect to database") {
		t.Fatalf("Open() error = %v, want a clear connection error", err)
	}
}

func TestNewMigratorRejectsUnsupportedDatabaseURL(t *testing.T) {
	tests := []struct {
		name        string
		databaseURL string
		wantInError string
	}{
		{
			name:        "unsupported scheme",
			databaseURL: "mysql://localhost/equipment",
			wantInError: "expected postgres or postgresql scheme",
		},
		{
			name:        "missing host",
			databaseURL: "postgres:///equipment",
			wantInError: "host is required",
		},
		{
			name:        "missing database name",
			databaseURL: "postgres://localhost",
			wantInError: "database name is required",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			migrator, err := NewMigrator(test.databaseURL, "../../migrations")
			if migrator != nil {
				migrator.Close()
				t.Fatal("NewMigrator() returned a migrator for an invalid database URL")
			}
			if err == nil || !strings.Contains(err.Error(), test.wantInError) {
				t.Fatalf("NewMigrator() error = %v, want it to contain %q", err, test.wantInError)
			}
		})
	}
}

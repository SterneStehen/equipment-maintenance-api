package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadValidConfiguration(t *testing.T) {
	setValidEnvironment(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.HTTPAddress != ":8080" {
		t.Errorf("HTTPAddress = %q, want %q", cfg.HTTPAddress, ":8080")
	}
	if cfg.DatabaseURL != "postgres://user:password@localhost:5432/equipment?sslmode=disable" {
		t.Errorf("DatabaseURL = %q", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "test-secret" {
		t.Errorf("JWTSecret = %q, want %q", cfg.JWTSecret, "test-secret")
	}
	if cfg.JWTTTL != 15*time.Minute {
		t.Errorf("JWTTTL = %s, want %s", cfg.JWTTTL, 15*time.Minute)
	}
	if cfg.DBMaxConnections != 10 {
		t.Errorf("DBMaxConnections = %d, want 10", cfg.DBMaxConnections)
	}
	if cfg.DBMinConnections != 2 {
		t.Errorf("DBMinConnections = %d, want 2", cfg.DBMinConnections)
	}
}

func TestLoadReportsMissingConfiguration(t *testing.T) {
	for _, name := range environmentVariableNames {
		t.Setenv(name, "")
	}

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}

	for _, name := range environmentVariableNames {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Load() error = %q, want it to mention %s", err, name)
		}
	}
}

func TestLoadRejectsInvalidConfiguration(t *testing.T) {
	setValidEnvironment(t)
	t.Setenv("HTTP_ADDRESS", "localhost")
	t.Setenv("DATABASE_URL", "http://localhost/equipment")
	t.Setenv("JWT_TTL", "forever")
	t.Setenv("DB_MAX_CONNECTIONS", "2")
	t.Setenv("DB_MIN_CONNECTIONS", "3")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want an error")
	}

	for _, expected := range []string{
		"HTTP_ADDRESS",
		"DATABASE_URL",
		"JWT_TTL",
		"DB_MIN_CONNECTIONS must not exceed DB_MAX_CONNECTIONS",
	} {
		if !strings.Contains(err.Error(), expected) {
			t.Errorf("Load() error = %q, want it to mention %q", err, expected)
		}
	}
}

var environmentVariableNames = []string{
	"HTTP_ADDRESS",
	"DATABASE_URL",
	"JWT_SECRET",
	"JWT_TTL",
	"DB_MAX_CONNECTIONS",
	"DB_MIN_CONNECTIONS",
}

func setValidEnvironment(t *testing.T) {
	t.Helper()
	t.Setenv("HTTP_ADDRESS", ":8080")
	t.Setenv("DATABASE_URL", "postgres://user:password@localhost:5432/equipment?sslmode=disable")
	t.Setenv("JWT_SECRET", "test-secret")
	t.Setenv("JWT_TTL", "15m")
	t.Setenv("DB_MAX_CONNECTIONS", "10")
	t.Setenv("DB_MIN_CONNECTIONS", "2")
}

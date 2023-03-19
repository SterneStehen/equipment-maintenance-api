package database

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func NewMigrator(databaseURL, migrationsPath string) (*migrate.Migrate, error) {
	absPath, err := filepath.Abs(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("resolve migrations path: %w", err)
	}

	// Tests don't always start in the repo root, so use the full path here
	filesURL := (&url.URL{Scheme: "file", Path: filepath.ToSlash(absPath)}).String()
	dbURL, err := url.Parse(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse migration database URL: %w", err)
	}
	if dbURL.Scheme != "postgres" && dbURL.Scheme != "postgresql" {
		return nil, fmt.Errorf("parse migration database URL: expected postgres or postgresql scheme")
	}
	if dbURL.Host == "" {
		return nil, fmt.Errorf("parse migration database URL: host is required")
	}
	if strings.Trim(dbURL.Path, "/") == "" {
		return nil, fmt.Errorf("parse migration database URL: database name is required")
	}
	dbURL.Scheme = "pgx"

	migrator, err := migrate.New(filesURL, dbURL.String())
	if err != nil {
		return nil, fmt.Errorf("initialize migrations: %w", err)
	}

	return migrator, nil
}

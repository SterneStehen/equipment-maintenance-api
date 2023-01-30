package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Config struct {
	URL            string
	MaxConnections int32
	MinConnections int32
}

func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	if strings.TrimSpace(cfg.URL) == "" {
		return nil, fmt.Errorf("configure database pool: URL is required")
	}
	if cfg.MaxConnections < 1 {
		return nil, fmt.Errorf("configure database pool: maximum connections must be at least 1")
	}
	if cfg.MinConnections < 0 {
		return nil, fmt.Errorf("configure database pool: minimum connections must not be negative")
	}
	if cfg.MinConnections > cfg.MaxConnections {
		return nil, fmt.Errorf("configure database pool: minimum connections must not exceed maximum connections")
	}

	// pgx accepts DSNs too; the outer config is stricter on purpose
	pgcfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database configuration: %w", err)
	}
	pgcfg.MaxConns = cfg.MaxConnections
	pgcfg.MinConns = cfg.MinConnections

	pool, err := pgxpool.ConnectConfig(ctx, pgcfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("verify database readiness: %w", err)
	}

	return pool, nil
}

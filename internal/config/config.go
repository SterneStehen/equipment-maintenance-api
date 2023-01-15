// Package config loads and validates application configuration from the environment.
package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config contains all process-level configuration required by the application.
type Config struct {
	HTTPAddress      string
	DatabaseURL      string
	JWTSecret        string
	JWTTTL           time.Duration
	DBMaxConnections int32
	DBMinConnections int32
}

// Load reads configuration from environment variables and validates the complete
// result so an operator can correct all startup errors in one pass.
func Load() (Config, error) {
	var cfg Config
	var problems []string

	cfg.HTTPAddress = strings.TrimSpace(os.Getenv("HTTP_ADDRESS"))
	if cfg.HTTPAddress == "" {
		problems = append(problems, "HTTP_ADDRESS is required")
	} else if err := validateHTTPAddress(cfg.HTTPAddress); err != nil {
		problems = append(problems, "HTTP_ADDRESS "+err.Error())
	}

	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		problems = append(problems, "DATABASE_URL is required")
	} else if err := validateDatabaseURL(cfg.DatabaseURL); err != nil {
		problems = append(problems, "DATABASE_URL "+err.Error())
	}

	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		problems = append(problems, "JWT_SECRET is required")
	}

	cfg.JWTTTL = parseDuration("JWT_TTL", &problems)
	cfg.DBMaxConnections = parseInt32("DB_MAX_CONNECTIONS", 1, &problems)
	cfg.DBMinConnections = parseInt32("DB_MIN_CONNECTIONS", 0, &problems)
	if cfg.DBMaxConnections > 0 && cfg.DBMinConnections > cfg.DBMaxConnections {
		problems = append(problems, "DB_MIN_CONNECTIONS must not exceed DB_MAX_CONNECTIONS")
	}

	if len(problems) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(problems, "; "))
	}

	return cfg, nil
}

func validateHTTPAddress(address string) error {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("must be in host:port form: %v", err)
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return fmt.Errorf("must contain a port from 1 to 65535")
	}

	return nil
}

func validateDatabaseURL(rawURL string) error {
	databaseURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("must be a valid PostgreSQL URL: %v", err)
	}
	if databaseURL.Scheme != "postgres" && databaseURL.Scheme != "postgresql" {
		return fmt.Errorf("must use the postgres or postgresql scheme")
	}
	if databaseURL.Host == "" {
		return fmt.Errorf("must include a host")
	}
	if strings.Trim(databaseURL.Path, "/") == "" {
		return fmt.Errorf("must include a database name")
	}

	return nil
}

func parseDuration(name string, problems *[]string) time.Duration {
	rawValue := strings.TrimSpace(os.Getenv(name))
	if rawValue == "" {
		*problems = append(*problems, name+" is required")
		return 0
	}

	value, err := time.ParseDuration(rawValue)
	if err != nil || value <= 0 {
		*problems = append(*problems, name+" must be a positive duration such as 15m")
		return 0
	}

	return value
}

func parseInt32(name string, minimum int32, problems *[]string) int32 {
	rawValue := strings.TrimSpace(os.Getenv(name))
	if rawValue == "" {
		*problems = append(*problems, name+" is required")
		return 0
	}

	value, err := strconv.ParseInt(rawValue, 10, 32)
	if err != nil || value < int64(minimum) {
		*problems = append(*problems, fmt.Sprintf("%s must be an integer greater than or equal to %d", name, minimum))
		return 0
	}

	return int32(value)
}

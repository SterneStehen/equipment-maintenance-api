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

type Config struct {
	HTTPAddress      string
	DatabaseURL      string
	JWTSecret        string
	JWTTTL           time.Duration
	DBMaxConnections int32
	DBMinConnections int32
}

func Load() (Config, error) {
	var cfg Config
	var issues []string

	cfg.HTTPAddress = strings.TrimSpace(os.Getenv("HTTP_ADDRESS"))
	if cfg.HTTPAddress == "" {
		issues = append(issues, "HTTP_ADDRESS is required")
	} else if err := checkHTTPAddr(cfg.HTTPAddress); err != nil {
		issues = append(issues, "HTTP_ADDRESS "+err.Error())
	}

	cfg.DatabaseURL = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if cfg.DatabaseURL == "" {
		issues = append(issues, "DATABASE_URL is required")
	} else if err := validDBURL(cfg.DatabaseURL); err != nil {
		issues = append(issues, "DATABASE_URL "+err.Error())
	}

	// Keep the raw secret
	cfg.JWTSecret = os.Getenv("JWT_SECRET")
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		issues = append(issues, "JWT_SECRET is required")
	}

	cfg.JWTTTL = parseDuration("JWT_TTL", &issues)
	cfg.DBMaxConnections = parseInt32("DB_MAX_CONNECTIONS", 1, &issues)
	cfg.DBMinConnections = parseInt32("DB_MIN_CONNECTIONS", 0, &issues)
	if cfg.DBMaxConnections > 0 && cfg.DBMinConnections > cfg.DBMaxConnections {
		issues = append(issues, "DB_MIN_CONNECTIONS must not exceed DB_MAX_CONNECTIONS")
	}

	// Report everything at once; fixing one env var per restart gets old fast
	if len(issues) > 0 {
		return Config{}, fmt.Errorf("invalid configuration: %s", strings.Join(issues, "; "))
	}

	return cfg, nil
}

func checkHTTPAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("must be in host:port form: %v", err)
	}

	portNum, err := strconv.Atoi(port)
	if err != nil || portNum < 1 || portNum > 65535 {
		return fmt.Errorf("must contain a port from 1 to 65535")
	}

	return nil
}

func validDBURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("must be a valid PostgreSQL URL: %v", err)
	}
	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return fmt.Errorf("must use the postgres or postgresql scheme")
	}
	if u.Host == "" {
		return fmt.Errorf("must include a host")
	}
	if strings.Trim(u.Path, "/") == "" {
		return fmt.Errorf("must include a database name")
	}

	return nil
}

func parseDuration(name string, issues *[]string) time.Duration {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		*issues = append(*issues, name+" is required")
		return 0
	}

	v, err := time.ParseDuration(raw)
	if err != nil || v <= 0 {
		*issues = append(*issues, name+" must be a positive duration such as 15m")
		return 0
	}

	return v
}

func parseInt32(name string, minimum int32, issues *[]string) int32 {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		*issues = append(*issues, name+" is required")
		return 0
	}

	v, err := strconv.ParseInt(raw, 10, 32)
	if err != nil || v < int64(minimum) {
		*issues = append(*issues, fmt.Sprintf("%s must be an integer greater than or equal to %d", name, minimum))
		return 0
	}

	return int32(v)
}

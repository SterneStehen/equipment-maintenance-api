package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	appdatabase "github.com/SterneStehen/equipment-maintenance-api/internal/database"
	"github.com/golang-migrate/migrate/v4"
)

func main() {
	logger := log.New(os.Stdout, "migrate: ", log.Ldate|log.Ltime|log.LUTC)
	if err := run(os.Args[1:], logger); err != nil {
		logger.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(args []string, logger *log.Logger) (runErr error) {
	flags := flag.NewFlagSet("migrate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	dir := flags.String("path", "migrations", "path to migration files")
	if err := flags.Parse(args); err != nil {
		return err
	}

	rest := flags.Args()
	if len(rest) == 0 {
		return fmt.Errorf("usage: migrate [-path migrations] <up|down|version> [steps]")
	}

	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required")
	}

	migrator, err := appdatabase.NewMigrator(databaseURL, *dir)
	if err != nil {
		return err
	}
	defer func() {
		sourceErr, databaseErr := migrator.Close()
		if runErr == nil && (sourceErr != nil || databaseErr != nil) {
			runErr = fmt.Errorf("close migrator: source error: %v; database error: %v", sourceErr, databaseErr)
		}
	}()

	switch rest[0] {
	case "up":
		if len(rest) != 1 {
			return fmt.Errorf("usage: migrate up")
		}
		err = migrator.Up()
	case "down":
		// A plain "down" is too easy to fat-finger, make the count explicit
		steps, parseErr := downSteps(rest)
		if parseErr != nil {
			return parseErr
		}
		err = migrator.Steps(-steps)
	case "version":
		if len(rest) != 1 {
			return fmt.Errorf("usage: migrate version")
		}
		version, dirty, versionErr := migrator.Version()
		if versionErr != nil {
			return fmt.Errorf("read migration version: %w", versionErr)
		}
		logger.Printf("version %d (dirty=%t)", version, dirty)
		return nil
	default:
		return fmt.Errorf("unknown command %q; expected up, down, or version", rest[0])
	}

	if errors.Is(err, migrate.ErrNoChange) {
		logger.Print("no migration change required")
		return nil
	}
	if err != nil {
		return fmt.Errorf("migrate %s: %w", rest[0], err)
	}

	logger.Printf("migration %s completed", rest[0])
	return nil
}

func downSteps(args []string) (int, error) {
	if len(args) != 2 {
		return 0, fmt.Errorf("usage: migrate down <positive-steps>")
	}
	steps, err := strconv.Atoi(args[1])
	if err != nil || steps < 1 {
		return 0, fmt.Errorf("down steps must be a positive integer")
	}
	return steps, nil
}

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/config"
	"github.com/SterneStehen/equipment-maintenance-api/internal/database"
	"github.com/SterneStehen/equipment-maintenance-api/internal/equipment"
	"github.com/SterneStehen/equipment-maintenance-api/internal/health"
	"github.com/SterneStehen/equipment-maintenance-api/internal/maintenance"
	"github.com/SterneStehen/equipment-maintenance-api/internal/server"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/SterneStehen/equipment-maintenance-api/internal/workorder"
)

func main() {
	logger := log.New(os.Stdout, "api: ", log.Ldate|log.Ltime|log.LUTC)
	if err := run(logger); err != nil {
		logger.Printf("error: %v", err)
		os.Exit(1)
	}
}

func run(logger *log.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dbCtx, cancelDB := context.WithTimeout(ctx, 10*time.Second)
	pool, err := database.Open(dbCtx, database.Config{
		URL:            cfg.DatabaseURL,
		MaxConnections: cfg.DBMaxConnections,
		MinConnections: cfg.DBMinConnections,
	})
	cancelDB()
	if err != nil {
		return fmt.Errorf("database startup error: %w", err)
	}
	defer pool.Close()
	logger.Print("database connection ready")

	users := user.NewService(user.NewRepository(pool))
	tokens := auth.NewManager(cfg.JWTSecret, cfg.JWTTTL)
	authHandler := auth.NewHandler(users, tokens)
	equipmentSvc := equipment.NewService(equipment.NewRepository(pool))
	equipmentHandler := equipment.NewHandler(equipmentSvc)
	workOrders := workorder.NewService(workorder.NewRepository(pool))
	workOrderHandler := workorder.NewHandler(workOrders)
	maintenanceSvc := maintenance.NewService(maintenance.NewRepository(pool))
	maintenanceHandler := maintenance.NewHandler(maintenanceSvc)
	router := server.NewRouter(server.Dependencies{
		Auth:      authHandler,
		Equipment: equipmentHandler,
		Logger:    logger,
		Maint:     maintenanceHandler,
		Ready:     health.NewReadyHandler(pool),
		Tokens:    tokens,
		Users:     users,
		WorkOrder: workOrderHandler,
	})

	httpServer := server.New(cfg.HTTPAddress, router)
	logger.Printf("listening on %s", cfg.HTTPAddress)
	if err := httpServer.Run(ctx); err != nil {
		return fmt.Errorf("HTTP server error: %w", err)
	}
	logger.Print("HTTP server stopped")

	return nil
}

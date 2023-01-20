package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/SterneStehen/equipment-maintenance-api/internal/config"
	"github.com/SterneStehen/equipment-maintenance-api/internal/server"
)

func main() {
	logger := log.New(os.Stdout, "api: ", log.Ldate|log.Ltime|log.LUTC)

	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("configuration error: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpServer := server.New(cfg.HTTPAddress, server.NewRouter())
	logger.Printf("listening on %s", cfg.HTTPAddress)
	if err := httpServer.Run(ctx); err != nil {
		logger.Fatalf("HTTP server error: %v", err)
	}
	logger.Print("HTTP server stopped")
}

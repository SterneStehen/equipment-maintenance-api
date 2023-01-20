package server

import (
	"net/http"

	"github.com/SterneStehen/equipment-maintenance-api/internal/health"
	"github.com/gin-gonic/gin"
)

// NewRouter builds the HTTP handler and registers all application routes.
func NewRouter() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", health.Handler)
	return router
}

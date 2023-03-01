package server

import (
	"net/http"

	"github.com/SterneStehen/equipment-maintenance-api/internal/health"
	"github.com/gin-gonic/gin"
)

func NewRouter() http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", health.Check)
	return router
}

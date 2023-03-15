package server

import (
	"net/http"

	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/health"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Auth   *auth.Handler
	Tokens *auth.Manager
}

func NewRouter(deps Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.GET("/health", health.Check)

	if deps.Auth != nil && deps.Tokens != nil {
		v1 := router.Group("/api/v1")
		v1.POST("/auth/register", deps.Auth.Register)
		v1.POST("/auth/login", deps.Auth.Login)
		v1.GET("/users/me", deps.Tokens.Middleware(), deps.Auth.Me)
	}
	return router
}

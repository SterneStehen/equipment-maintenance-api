package server

import (
	"net/http"

	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/health"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
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
		protected := v1.Group("", deps.Tokens.Middleware())
		protected.GET("/users/me", deps.Auth.Me)

		admins := protected.Group("", auth.RequireRole(user.RoleAdmin))
		admins.GET("/admin/users", deps.Auth.ListUsers)
		admins.GET("/admin/users/:id", deps.Auth.GetUser)
		admins.PATCH("/admin/users/:id/role", deps.Auth.UpdateUserRole)
	}
	return router
}

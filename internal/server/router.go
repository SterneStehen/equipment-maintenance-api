package server

import (
	"net/http"

	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/equipment"
	"github.com/SterneStehen/equipment-maintenance-api/internal/health"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/SterneStehen/equipment-maintenance-api/internal/workorder"
	"github.com/gin-gonic/gin"
)

type Dependencies struct {
	Auth      *auth.Handler
	Equipment *equipment.Handler
	Tokens    *auth.Manager
	WorkOrder *workorder.Handler
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

		if deps.Equipment != nil {
			protected.GET("/equipment", deps.Equipment.List)
			protected.GET("/equipment/:id", deps.Equipment.Get)
			protected.DELETE("/equipment/:id", deps.Equipment.Delete)

			eqWrite := protected.Group("", auth.RequireRole(user.RoleAdmin, user.RoleDispatcher))
			eqWrite.POST("/equipment", deps.Equipment.Create)
			eqWrite.PATCH("/equipment/:id", deps.Equipment.Update)

			admins.POST("/equipment/:id/decommission", deps.Equipment.Decommission)
		}
		if deps.WorkOrder != nil {
			protected.GET("/work-orders", deps.WorkOrder.List)
			protected.GET("/work-orders/:id", deps.WorkOrder.Get)

			woWrite := protected.Group("", auth.RequireRole(user.RoleAdmin, user.RoleDispatcher))
			woWrite.POST("/work-orders", deps.WorkOrder.Create)
			woWrite.PATCH("/work-orders/:id", deps.WorkOrder.Update)
		}
	}
	return router
}

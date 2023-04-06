package auth

import (
	"net/http"
	"strings"

	"github.com/SterneStehen/equipment-maintenance-api/internal/apperror"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/gin-gonic/gin"
)

const principalKey = "auth.principal"

func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Fields handles the extra spaces people somehow manage to send
		bits := strings.Fields(c.GetHeader("Authorization"))
		if len(bits) != 2 || !strings.EqualFold(bits[0], "Bearer") {
			apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authentication is required")
			c.Abort()
			return
		}

		who, err := m.Verify(bits[1])
		if err != nil {
			apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Access token is invalid or expired")
			c.Abort()
			return
		}
		c.Set(principalKey, who)
		c.Next()
	}
}

func Current(c *gin.Context) (Principal, bool) {
	v, ok := c.Get(principalKey)
	if !ok {
		return Principal{}, false
	}
	p, ok := v.(Principal)
	return p, ok
}

func RequireRole(roles ...user.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		p, ok := Current(c)
		if !ok {
			apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authentication is required")
			c.Abort()
			return
		}
		for i := range roles {
			if p.Role == roles[i] {
				c.Next()
				return
			}
		}
		apperror.Write(c, http.StatusForbidden, "forbidden", "You do not have permission to use this endpoint")
		c.Abort()
	}
}

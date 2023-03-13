package auth

import (
	"net/http"
	"strings"

	"github.com/SterneStehen/equipment-maintenance-api/internal/apperror"
	"github.com/gin-gonic/gin"
)

const principalKey = "auth.principal"

func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Fields also deals with the weird extra spaces clients sometimes send
		bits := strings.Fields(c.GetHeader("Authorization"))
		if len(bits) != 2 || !strings.EqualFold(bits[0], "Bearer") {
			apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Authentication is required")
			return
		}

		who, err := m.Verify(bits[1])
		if err != nil {
			apperror.Write(c, http.StatusUnauthorized, "unauthorized", "Access token is invalid or expired")
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

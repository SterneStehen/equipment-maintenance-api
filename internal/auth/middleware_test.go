package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type freshUsers struct {
	u   user.User
	err error
}

func (f freshUsers) ByID(context.Context, int64) (user.User, error) {
	return f.u, f.err
}

func TestFreshUserReplacesRoleFromStore(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	mgr := NewManager("fresh-secret", 0)
	router.Use(func(c *gin.Context) {
		c.Set(principalKey, Principal{UserID: 7, Role: user.RoleViewer})
		c.Next()
	}, FreshUser(freshUsers{u: user.User{ID: 7, Role: user.RoleDispatcher}}))
	router.GET("/", func(c *gin.Context) {
		p, ok := Current(c)
		require.True(t, ok)
		c.String(http.StatusOK, string(p.Role))
	})
	_ = mgr

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, "dispatcher", res.Body.String())
}

func TestFreshUserRejectsMissingUser(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set(principalKey, Principal{UserID: 7, Role: user.RoleViewer})
		c.Next()
	}, FreshUser(freshUsers{err: user.ErrNotFound}))
	router.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "nope")
	})

	res := httptest.NewRecorder()
	router.ServeHTTP(res, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, http.StatusUnauthorized, res.Code)
}

package maintenance_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/maintenance"
	"github.com/SterneStehen/equipment-maintenance-api/internal/server"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMaint struct {
	listFn func(context.Context, user.Actor, maintenance.ListFilter) ([]maintenance.Record, error)
}

func (f fakeMaint) List(ctx context.Context, a user.Actor, flt maintenance.ListFilter) ([]maintenance.Record, error) {
	return f.listFn(ctx, a, flt)
}

func TestMaintenanceListRoute(t *testing.T) {
	var got maintenance.ListFilter
	api := fakeMaint{listFn: func(_ context.Context, a user.Actor, flt maintenance.ListFilter) ([]maintenance.Record, error) {
		require.Equal(t, user.RoleViewer, a.Role)
		got = flt
		return []maintenance.Record{{ID: 2, WorkOrderID: 7, EquipmentID: 4, PerformedBy: a.UserID, Notes: "done"}}, nil
	}}
	router, secret := maintRouter(api)
	viewerTok := token(t, secret, user.RoleViewer)

	res := hit(router, http.MethodGet, "/api/v1/maintenance-records?work_order_id=7&equipment_id=4&limit=3&offset=1", "", "Bearer "+viewerTok)
	require.Equal(t, http.StatusOK, res.Code)
	assert.Equal(t, int64(7), got.WorkOrderID)
	assert.Equal(t, 3, got.Limit)
	assert.Contains(t, res.Body.String(), `"maintenance_records"`)
}

func TestMaintenanceRouteErrors(t *testing.T) {
	api := fakeMaint{listFn: func(context.Context, user.Actor, maintenance.ListFilter) ([]maintenance.Record, error) {
		return nil, maintenance.ErrPermissionDenied
	}}
	router, secret := maintRouter(api)
	viewerTok := token(t, secret, user.RoleViewer)

	bad := hit(router, http.MethodGet, "/api/v1/maintenance-records?performed_by=nope", "", "Bearer "+viewerTok)
	assert.Equal(t, http.StatusBadRequest, bad.Code)

	forbidden := hit(router, http.MethodGet, "/api/v1/maintenance-records", "", "Bearer "+viewerTok)
	assert.Equal(t, http.StatusForbidden, forbidden.Code)

	missing := hit(router, http.MethodGet, "/api/v1/maintenance-records", "", "")
	assert.Equal(t, http.StatusUnauthorized, missing.Code)
}

type authStub struct{}

func (authStub) Register(context.Context, user.RegisterInput) (user.User, error) {
	return user.User{}, nil
}
func (authStub) Authenticate(context.Context, string, string) (user.User, error) {
	return user.User{}, nil
}
func (authStub) ByID(context.Context, int64) (user.User, error) { return user.User{}, nil }
func (authStub) List(context.Context, user.Actor) ([]user.User, error) {
	return nil, nil
}
func (authStub) Lookup(context.Context, user.Actor, int64) (user.User, error) {
	return user.User{}, nil
}
func (authStub) AssignRole(context.Context, user.Actor, int64, user.Role) (user.User, error) {
	return user.User{}, nil
}

func maintRouter(api fakeMaint) (http.Handler, string) {
	secret := "maintenance-handler-secret"
	tokens := auth.NewManager(secret, time.Minute)
	return server.NewRouter(server.Dependencies{
		Auth: auth.NewHandler(authStub{}, tokens), Tokens: tokens, Maint: maintenance.NewHandler(api),
	}), secret
}

func token(t *testing.T, secret string, role user.Role) string {
	t.Helper()
	raw, _, err := auth.NewManager(secret, time.Minute).Issue(user.User{ID: 14, Role: role})
	require.NoError(t, err)
	return raw
}

func hit(h http.Handler, method, path, body, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(authorization) != "" {
		req.Header.Set("Authorization", authorization)
	}
	res := httptest.NewRecorder()
	h.ServeHTTP(res, req)
	return res
}

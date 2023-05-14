package workorder_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/server"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/SterneStehen/equipment-maintenance-api/internal/workorder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeWO struct {
	createFn func(context.Context, user.Actor, workorder.CreateInput) (workorder.WorkOrder, error)
	byIDFn   func(context.Context, user.Actor, int64) (workorder.WorkOrder, error)
	listFn   func(context.Context, user.Actor, workorder.ListFilter) ([]workorder.WorkOrder, error)
	updateFn func(context.Context, user.Actor, int64, workorder.UpdateInput) (workorder.WorkOrder, error)
}

func (f fakeWO) Create(ctx context.Context, a user.Actor, in workorder.CreateInput) (workorder.WorkOrder, error) {
	return f.createFn(ctx, a, in)
}

func (f fakeWO) ByID(ctx context.Context, a user.Actor, id int64) (workorder.WorkOrder, error) {
	return f.byIDFn(ctx, a, id)
}

func (f fakeWO) List(ctx context.Context, a user.Actor, flt workorder.ListFilter) ([]workorder.WorkOrder, error) {
	return f.listFn(ctx, a, flt)
}

func (f fakeWO) Update(ctx context.Context, a user.Actor, id int64, in workorder.UpdateInput) (workorder.WorkOrder, error) {
	return f.updateFn(ctx, a, id, in)
}

func TestWorkOrderCreateAndReadRoutes(t *testing.T) {
	var got workorder.CreateInput
	api := fakeWO{
		createFn: func(_ context.Context, a user.Actor, in workorder.CreateInput) (workorder.WorkOrder, error) {
			require.Equal(t, user.RoleDispatcher, a.Role)
			got = in
			x := sampleWO()
			x.Title = in.Title
			return x, nil
		},
		byIDFn: func(_ context.Context, a user.Actor, id int64) (workorder.WorkOrder, error) {
			require.Equal(t, user.RoleViewer, a.Role)
			require.Equal(t, int64(8), id)
			return sampleWO(), nil
		},
		listFn: func(_ context.Context, a user.Actor, flt workorder.ListFilter) ([]workorder.WorkOrder, error) {
			require.Equal(t, workorder.StatusOpen, flt.Status)
			require.Equal(t, workorder.PriorityHigh, flt.Priority)
			require.Equal(t, int64(2), flt.EquipmentID)
			require.Equal(t, 3, flt.Limit)
			return []workorder.WorkOrder{sampleWO()}, nil
		},
	}
	router, secret := woRouter(api)
	viewerTok := token(t, secret, user.RoleViewer)
	dispatcherTok := token(t, secret, user.RoleDispatcher)

	blocked := hit(router, http.MethodPost, "/api/v1/work-orders", `{"equipment_id":2,"title":"Fix"}`, "Bearer "+viewerTok)
	assert.Equal(t, http.StatusForbidden, blocked.Code)

	created := hit(router, http.MethodPost, "/api/v1/work-orders", `{"equipment_id":2,"title":"Fix belt","priority":"high"}`, "Bearer "+dispatcherTok)
	require.Equal(t, http.StatusCreated, created.Code)
	assert.Equal(t, int64(2), got.EquipmentID)
	assert.Equal(t, "Fix belt", got.Title)

	found := hit(router, http.MethodGet, "/api/v1/work-orders/8", "", "Bearer "+viewerTok)
	require.Equal(t, http.StatusOK, found.Code)
	assert.Contains(t, found.Body.String(), `"title":"Fix pump"`)

	listed := hit(router, http.MethodGet, "/api/v1/work-orders?status=open&priority=high&equipment_id=2&limit=3", "", "Bearer "+viewerTok)
	require.Equal(t, http.StatusOK, listed.Code)
	assert.Contains(t, listed.Body.String(), `"work_orders"`)
}

func TestWorkOrderUpdateAndErrors(t *testing.T) {
	api := fakeWO{
		createFn: func(context.Context, user.Actor, workorder.CreateInput) (workorder.WorkOrder, error) {
			return workorder.WorkOrder{}, workorder.ErrEquipmentDecommissioned
		},
		updateFn: func(_ context.Context, a user.Actor, id int64, in workorder.UpdateInput) (workorder.WorkOrder, error) {
			require.Equal(t, user.RoleAdmin, a.Role)
			x := sampleWO()
			x.ID = id
			x.Status = in.Status
			return x, nil
		},
	}
	router, secret := woRouter(api)
	adminTok := token(t, secret, user.RoleAdmin)
	dispatcherTok := token(t, secret, user.RoleDispatcher)

	retired := hit(router, http.MethodPost, "/api/v1/work-orders", `{"equipment_id":2,"title":"Fix"}`, "Bearer "+dispatcherTok)
	assert.Equal(t, http.StatusConflict, retired.Code)
	assert.Contains(t, retired.Body.String(), `"code":"equipment_decommissioned"`)

	updated := hit(router, http.MethodPatch, "/api/v1/work-orders/9", `{"title":"Fix","status":"completed","priority":"urgent"}`, "Bearer "+adminTok)
	require.Equal(t, http.StatusOK, updated.Code)
	assert.Contains(t, updated.Body.String(), `"status":"completed"`)

	missingAuth := hit(router, http.MethodGet, "/api/v1/work-orders/9", "", "")
	assert.Equal(t, http.StatusUnauthorized, missingAuth.Code)
}

type authStub struct{}

func (authStub) Register(context.Context, user.RegisterInput) (user.User, error) { return user.User{}, nil }
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

func woRouter(api fakeWO) (http.Handler, string) {
	secret := "work-order-test-secret"
	tokens := auth.NewManager(secret, time.Minute)
	return server.NewRouter(server.Dependencies{
		Auth: auth.NewHandler(authStub{}, tokens), Tokens: tokens, WorkOrder: workorder.NewHandler(api),
	}), secret
}

func token(t *testing.T, secret string, role user.Role) string {
	t.Helper()
	raw, _, err := auth.NewManager(secret, time.Minute).Issue(user.User{ID: 11, Role: role})
	require.NoError(t, err)
	return raw
}

func sampleWO() workorder.WorkOrder {
	return workorder.WorkOrder{
		ID: 8, EquipmentID: 2, Title: "Fix pump", Status: workorder.StatusOpen,
		Priority: workorder.PriorityHigh, CreatedBy: 11,
		CreatedAt: time.Date(2023, 5, 10, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2023, 5, 10, 8, 0, 0, 0, time.UTC),
	}
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

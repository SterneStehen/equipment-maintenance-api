package equipment_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/equipment"
	"github.com/SterneStehen/equipment-maintenance-api/internal/server"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeEq struct {
	createFn       func(context.Context, user.Actor, equipment.CreateInput) (equipment.Equipment, error)
	byIDFn         func(context.Context, user.Actor, int64) (equipment.Equipment, error)
	listFn         func(context.Context, user.Actor, equipment.ListFilter) ([]equipment.Equipment, error)
	updateFn       func(context.Context, user.Actor, int64, equipment.UpdateInput) (equipment.Equipment, error)
	decommissionFn func(context.Context, user.Actor, int64) (equipment.Equipment, error)
}

func (f fakeEq) Create(ctx context.Context, a user.Actor, in equipment.CreateInput) (equipment.Equipment, error) {
	return f.createFn(ctx, a, in)
}

func (f fakeEq) ByID(ctx context.Context, a user.Actor, id int64) (equipment.Equipment, error) {
	return f.byIDFn(ctx, a, id)
}

func (f fakeEq) List(ctx context.Context, a user.Actor, flt equipment.ListFilter) ([]equipment.Equipment, error) {
	return f.listFn(ctx, a, flt)
}

func (f fakeEq) Update(ctx context.Context, a user.Actor, id int64, in equipment.UpdateInput) (equipment.Equipment, error) {
	return f.updateFn(ctx, a, id, in)
}

func (f fakeEq) Decommission(ctx context.Context, a user.Actor, id int64) (equipment.Equipment, error) {
	return f.decommissionFn(ctx, a, id)
}

func TestEquipmentCreateAndReadRoutes(t *testing.T) {
	var got equipment.CreateInput
	eq := fakeEq{
		createFn: func(_ context.Context, a user.Actor, in equipment.CreateInput) (equipment.Equipment, error) {
			require.Equal(t, user.RoleDispatcher, a.Role)
			got = in
			x := sampleEq()
			x.SerialNumber = "PUMP-9"
			return x, nil
		},
		listFn: func(_ context.Context, a user.Actor, flt equipment.ListFilter) ([]equipment.Equipment, error) {
			require.Equal(t, user.RoleViewer, a.Role)
			assert.Equal(t, equipment.StatusActive, flt.Status)
			assert.Equal(t, "pump", flt.Query)
			assert.Equal(t, 2, flt.Limit)
			assert.Equal(t, 1, flt.Offset)
			return []equipment.Equipment{sampleEq()}, nil
		},
		byIDFn: func(_ context.Context, a user.Actor, id int64) (equipment.Equipment, error) {
			require.Equal(t, int64(4), id)
			return sampleEq(), nil
		},
	}
	router, secret := eqRouter(eq)

	viewerTok := tok(t, secret, user.RoleViewer)
	dispatcherTok := tok(t, secret, user.RoleDispatcher)

	nope := hit(router, http.MethodPost, "/api/v1/equipment", `{"serial_number":"p-1","name":"Pump"}`, "Bearer "+viewerTok)
	assert.Equal(t, http.StatusForbidden, nope.Code)

	created := hit(router, http.MethodPost, "/api/v1/equipment", `{"serial_number":"p-1","name":"Pump","model":"M1"}`, "Bearer "+dispatcherTok)
	require.Equal(t, http.StatusCreated, created.Code)
	assert.Equal(t, "p-1", got.SerialNumber)
	assert.Contains(t, created.Body.String(), `"serial_number":"PUMP-9"`)

	listed := hit(router, http.MethodGet, "/api/v1/equipment?status=active&q=pump&limit=2&offset=1", "", "Bearer "+viewerTok)
	require.Equal(t, http.StatusOK, listed.Code)
	assert.Contains(t, listed.Body.String(), `"equipment"`)
	assert.Contains(t, listed.Body.String(), `"pagination"`)
	assert.Contains(t, listed.Body.String(), `"count":1`)

	found := hit(router, http.MethodGet, "/api/v1/equipment/4", "", "Bearer "+viewerTok)
	require.Equal(t, http.StatusOK, found.Code)
	assert.Contains(t, found.Body.String(), `"name":"Pump"`)
}

func TestEquipmentErrorsAndDecommissionRoutes(t *testing.T) {
	eq := fakeEq{
		createFn: func(context.Context, user.Actor, equipment.CreateInput) (equipment.Equipment, error) {
			return equipment.Equipment{}, equipment.ErrSerialTaken
		},
		updateFn: func(_ context.Context, a user.Actor, id int64, in equipment.UpdateInput) (equipment.Equipment, error) {
			require.Equal(t, user.RoleDispatcher, a.Role)
			x := sampleEq()
			x.ID = id
			x.Status = in.Status
			return x, nil
		},
		decommissionFn: func(_ context.Context, a user.Actor, id int64) (equipment.Equipment, error) {
			require.Equal(t, user.RoleAdmin, a.Role)
			x := sampleEq()
			x.ID = id
			x.Status = equipment.StatusDecommissioned
			now := time.Now().UTC()
			x.DecommissionedAt = &now
			return x, nil
		},
	}
	router, secret := eqRouter(eq)
	adminTok := tok(t, secret, user.RoleAdmin)
	dispatcherTok := tok(t, secret, user.RoleDispatcher)

	dup := hit(router, http.MethodPost, "/api/v1/equipment", `{"serial_number":"P-1","name":"Pump"}`, "Bearer "+dispatcherTok)
	assert.Equal(t, http.StatusConflict, dup.Code)
	assert.Contains(t, dup.Body.String(), `"code":"serial_number_exists"`)

	updated := hit(router, http.MethodPatch, "/api/v1/equipment/7", `{"name":"Pump","status":"maintenance"}`, "Bearer "+dispatcherTok)
	require.Equal(t, http.StatusOK, updated.Code)
	assert.Contains(t, updated.Body.String(), `"status":"maintenance"`)

	blocked := hit(router, http.MethodPost, "/api/v1/equipment/7/decommission", "", "Bearer "+dispatcherTok)
	assert.Equal(t, http.StatusForbidden, blocked.Code)

	done := hit(router, http.MethodPost, "/api/v1/equipment/7/decommission", "", "Bearer "+adminTok)
	require.Equal(t, http.StatusOK, done.Code)
	assert.Contains(t, done.Body.String(), `"status":"decommissioned"`)

	del := hit(router, http.MethodDelete, "/api/v1/equipment/7", "", "Bearer "+adminTok)
	assert.Equal(t, http.StatusMethodNotAllowed, del.Code)
	assert.Contains(t, del.Body.String(), `"code":"not_supported"`)
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

func eqRouter(eq fakeEq) (http.Handler, string) {
	secret := "equipment-handler-secret"
	tokens := auth.NewManager(secret, time.Minute)
	return server.NewRouter(server.Dependencies{
		Auth:      auth.NewHandler(authStub{}, tokens),
		Equipment: equipment.NewHandler(eq),
		Tokens:    tokens,
	}), secret
}

func tok(t *testing.T, secret string, role user.Role) string {
	t.Helper()
	raw, _, err := auth.NewManager(secret, time.Minute).Issue(user.User{ID: 12, Role: role})
	require.NoError(t, err)
	return raw
}

func sampleEq() equipment.Equipment {
	return equipment.Equipment{
		ID: 4, SerialNumber: "PUMP-4", Name: "Pump", Model: "M1",
		Location: "north", Status: equipment.StatusActive,
		CreatedAt: time.Date(2023, 3, 20, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2023, 3, 20, 8, 0, 0, 0, time.UTC),
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

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
	createFn   func(context.Context, user.Actor, workorder.CreateInput) (workorder.WorkOrder, error)
	byIDFn     func(context.Context, user.Actor, int64) (workorder.WorkOrder, error)
	listFn     func(context.Context, user.Actor, workorder.ListFilter) ([]workorder.WorkOrder, error)
	updateFn   func(context.Context, user.Actor, int64, workorder.UpdateInput) (workorder.WorkOrder, error)
	startFn    func(context.Context, user.Actor, int64, string) (workorder.WorkOrder, error)
	completeFn func(context.Context, user.Actor, int64, string) (workorder.WorkOrder, error)
	closeFn    func(context.Context, user.Actor, int64, string) (workorder.WorkOrder, error)
	cancelFn   func(context.Context, user.Actor, int64, string) (workorder.WorkOrder, error)
	commentFn  func(context.Context, user.Actor, int64, string) (workorder.Comment, error)
	commentsFn func(context.Context, user.Actor, int64, int, int) ([]workorder.Comment, error)
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

func (f fakeWO) Start(ctx context.Context, a user.Actor, id int64, note string) (workorder.WorkOrder, error) {
	return f.startFn(ctx, a, id, note)
}

func (f fakeWO) Complete(ctx context.Context, a user.Actor, id int64, note string) (workorder.WorkOrder, error) {
	return f.completeFn(ctx, a, id, note)
}

func (f fakeWO) Close(ctx context.Context, a user.Actor, id int64, note string) (workorder.WorkOrder, error) {
	return f.closeFn(ctx, a, id, note)
}

func (f fakeWO) Cancel(ctx context.Context, a user.Actor, id int64, note string) (workorder.WorkOrder, error) {
	return f.cancelFn(ctx, a, id, note)
}

func (f fakeWO) AddComment(ctx context.Context, a user.Actor, id int64, body string) (workorder.Comment, error) {
	return f.commentFn(ctx, a, id, body)
}

func (f fakeWO) ListComments(ctx context.Context, a user.Actor, id int64, limit, offset int) ([]workorder.Comment, error) {
	return f.commentsFn(ctx, a, id, limit, offset)
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
	assert.Contains(t, listed.Body.String(), `"pagination"`)
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
			x.Priority = in.Priority
			return x, nil
		},
	}
	router, secret := woRouter(api)
	adminTok := token(t, secret, user.RoleAdmin)
	dispatcherTok := token(t, secret, user.RoleDispatcher)

	retired := hit(router, http.MethodPost, "/api/v1/work-orders", `{"equipment_id":2,"title":"Fix"}`, "Bearer "+dispatcherTok)
	assert.Equal(t, http.StatusConflict, retired.Code)
	assert.Contains(t, retired.Body.String(), `"code":"equipment_decommissioned"`)

	updated := hit(router, http.MethodPatch, "/api/v1/work-orders/9", `{"title":"Fix","priority":"urgent"}`, "Bearer "+adminTok)
	require.Equal(t, http.StatusOK, updated.Code)
	assert.Contains(t, updated.Body.String(), `"priority":"urgent"`)

	missingAuth := hit(router, http.MethodGet, "/api/v1/work-orders/9", "", "")
	assert.Equal(t, http.StatusUnauthorized, missingAuth.Code)
}

func TestWorkOrderTransitionRoutes(t *testing.T) {
	var gotNote string
	api := fakeWO{
		startFn: func(_ context.Context, a user.Actor, id int64, note string) (workorder.WorkOrder, error) {
			require.Equal(t, user.RoleTechnician, a.Role)
			require.Equal(t, int64(9), id)
			gotNote = note
			x := sampleWO()
			x.Status = workorder.StatusInProgress
			return x, nil
		},
		completeFn: func(context.Context, user.Actor, int64, string) (workorder.WorkOrder, error) {
			return workorder.WorkOrder{}, workorder.ErrTechnicianOwnership
		},
		closeFn: func(context.Context, user.Actor, int64, string) (workorder.WorkOrder, error) {
			x := sampleWO()
			x.Status = workorder.StatusClosed
			return x, nil
		},
		cancelFn: func(context.Context, user.Actor, int64, string) (workorder.WorkOrder, error) {
			return workorder.WorkOrder{}, workorder.ErrInvalidTransition
		},
	}
	router, secret := woRouter(api)
	techTok := token(t, secret, user.RoleTechnician)
	adminTok := token(t, secret, user.RoleAdmin)

	started := hit(router, http.MethodPost, "/api/v1/work-orders/9/start", `{"note":"on it"}`, "Bearer "+techTok)
	require.Equal(t, http.StatusOK, started.Code)
	assert.Equal(t, "on it", gotNote)
	assert.Contains(t, started.Body.String(), `"status":"in_progress"`)

	owned := hit(router, http.MethodPost, "/api/v1/work-orders/9/complete", `{}`, "Bearer "+techTok)
	assert.Equal(t, http.StatusForbidden, owned.Code)
	assert.Contains(t, owned.Body.String(), `"code":"not_assigned"`)

	closed := hit(router, http.MethodPost, "/api/v1/work-orders/9/close", "", "Bearer "+adminTok)
	assert.Equal(t, http.StatusOK, closed.Code)

	bad := hit(router, http.MethodPost, "/api/v1/work-orders/9/cancel", "", "Bearer "+adminTok)
	assert.Equal(t, http.StatusConflict, bad.Code)
}

func TestWorkOrderAssigneeError(t *testing.T) {
	api := fakeWO{
		createFn: func(context.Context, user.Actor, workorder.CreateInput) (workorder.WorkOrder, error) {
			return workorder.WorkOrder{}, workorder.ErrAssigneeNotTechnician
		},
	}
	router, secret := woRouter(api)
	dispatcherTok := token(t, secret, user.RoleDispatcher)

	res := hit(router, http.MethodPost, "/api/v1/work-orders", `{"equipment_id":2,"title":"Fix","assigned_to":4}`, "Bearer "+dispatcherTok)
	assert.Equal(t, http.StatusBadRequest, res.Code)
	assert.Contains(t, res.Body.String(), `"code":"invalid_assignee"`)
}

func TestWorkOrderCommentsRoutes(t *testing.T) {
	api := fakeWO{
		commentFn: func(_ context.Context, a user.Actor, id int64, body string) (workorder.Comment, error) {
			require.Equal(t, user.RoleViewer, a.Role)
			require.Equal(t, int64(9), id)
			require.Equal(t, "hi", body)
			return workorder.Comment{ID: 3, WorkOrderID: id, AuthorID: a.UserID, Body: body}, nil
		},
		commentsFn: func(_ context.Context, a user.Actor, id int64, limit, offset int) ([]workorder.Comment, error) {
			require.Equal(t, int64(9), id)
			require.Equal(t, 2, limit)
			require.Equal(t, 1, offset)
			return []workorder.Comment{{ID: 3, WorkOrderID: id, Body: "hi"}}, nil
		},
	}
	router, secret := woRouter(api)
	viewerTok := token(t, secret, user.RoleViewer)

	created := hit(router, http.MethodPost, "/api/v1/work-orders/9/comments", `{"body":"hi"}`, "Bearer "+viewerTok)
	require.Equal(t, http.StatusCreated, created.Code)
	assert.Contains(t, created.Body.String(), `"body":"hi"`)

	listed := hit(router, http.MethodGet, "/api/v1/work-orders/9/comments?limit=2&offset=1", "", "Bearer "+viewerTok)
	require.Equal(t, http.StatusOK, listed.Code)
	assert.Contains(t, listed.Body.String(), `"comments"`)
	assert.Contains(t, listed.Body.String(), `"count":1`)
}

func TestWorkOrderCommentError(t *testing.T) {
	api := fakeWO{
		commentFn: func(context.Context, user.Actor, int64, string) (workorder.Comment, error) {
			return workorder.Comment{}, workorder.ErrInvalidComment
		},
	}
	router, secret := woRouter(api)
	viewerTok := token(t, secret, user.RoleViewer)

	res := hit(router, http.MethodPost, "/api/v1/work-orders/9/comments", `{"body":" "}`, "Bearer "+viewerTok)
	assert.Equal(t, http.StatusBadRequest, res.Code)
	assert.Contains(t, res.Body.String(), `"code":"invalid_comment"`)
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

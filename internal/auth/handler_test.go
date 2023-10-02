package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/SterneStehen/equipment-maintenance-api/internal/auth"
	"github.com/SterneStehen/equipment-maintenance-api/internal/server"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUsers struct {
	registerFn     func(context.Context, user.RegisterInput) (user.User, error)
	authenticateFn func(context.Context, string, string) (user.User, error)
	byIDFn         func(context.Context, int64) (user.User, error)
	listFn         func(context.Context, user.Actor) ([]user.User, error)
	lookupFn       func(context.Context, user.Actor, int64) (user.User, error)
	assignRoleFn   func(context.Context, user.Actor, int64, user.Role) (user.User, error)
}

func (f fakeUsers) Register(ctx context.Context, in user.RegisterInput) (user.User, error) {
	return f.registerFn(ctx, in)
}

func (f fakeUsers) Authenticate(ctx context.Context, email, password string) (user.User, error) {
	return f.authenticateFn(ctx, email, password)
}

func (f fakeUsers) ByID(ctx context.Context, id int64) (user.User, error) {
	return f.byIDFn(ctx, id)
}

func (f fakeUsers) List(ctx context.Context, actor user.Actor) ([]user.User, error) {
	return f.listFn(ctx, actor)
}

func (f fakeUsers) Lookup(ctx context.Context, actor user.Actor, id int64) (user.User, error) {
	return f.lookupFn(ctx, actor, id)
}

func (f fakeUsers) AssignRole(ctx context.Context, actor user.Actor, id int64, role user.Role) (user.User, error) {
	return f.assignRoleFn(ctx, actor, id, role)
}

func TestRegisterHandlerIgnoresRoleAndHidesPassword(t *testing.T) {
	var got user.RegisterInput
	u := sampleUser()
	users := fakeUsers{registerFn: func(_ context.Context, in user.RegisterInput) (user.User, error) {
		got = in
		return u, nil
	}}
	router, _ := testRouter(users)

	res := perform(router, http.MethodPost, "/api/v1/auth/register", `{
		"email":"person@example.com","password":"password1","full_name":"Pat Smith","role":"admin"
	}`, "")

	require.Equal(t, http.StatusCreated, res.Code)
	assert.Equal(t, "person@example.com", got.Email)
	assert.Equal(t, "password1", got.Password)
	assert.NotContains(t, res.Body.String(), "password1")
	assert.NotContains(t, res.Body.String(), "password_hash")
	assert.NotContains(t, res.Body.String(), "stored-password-hash")
	assert.Contains(t, res.Body.String(), `"role":"viewer"`)
}

func TestLoginAndCurrentUser(t *testing.T) {
	u := sampleUser()
	users := fakeUsers{
		authenticateFn: func(_ context.Context, email, password string) (user.User, error) {
			require.Equal(t, "person@example.com", email)
			require.Equal(t, "password1", password)
			return u, nil
		},
		byIDFn: func(_ context.Context, id int64) (user.User, error) {
			require.Equal(t, u.ID, id)
			return u, nil
		},
	}
	router, secret := testRouter(users)

	login := perform(router, http.MethodPost, "/api/v1/auth/login", `{"email":"person@example.com","password":"password1"}`, "")
	require.Equal(t, http.StatusOK, login.Code)
	assert.NotContains(t, login.Body.String(), secret)
	assert.NotContains(t, login.Body.String(), "stored-password-hash")

	var body struct {
		AccessToken string `json:"access_token"`
	}
	require.NoError(t, json.Unmarshal(login.Body.Bytes(), &body))
	require.NotEmpty(t, body.AccessToken)

	me := perform(router, http.MethodGet, "/api/v1/users/me", "", "Bearer "+body.AccessToken)
	require.Equal(t, http.StatusOK, me.Code)
	assert.Contains(t, me.Body.String(), `"email":"person@example.com"`)
	assert.NotContains(t, me.Body.String(), "stored-password-hash")
}

func TestProtectedRouteRejectsMissingAndInvalidTokens(t *testing.T) {
	router, _ := testRouter(fakeUsers{})

	missing := perform(router, http.MethodGet, "/api/v1/users/me", "", "")
	assert.Equal(t, http.StatusUnauthorized, missing.Code)
	assert.JSONEq(t, `{"error":{"code":"unauthorized","message":"Authentication is required"}}`, missing.Body.String())

	invalid := perform(router, http.MethodGet, "/api/v1/users/me", "", "Bearer broken.token.value")
	assert.Equal(t, http.StatusUnauthorized, invalid.Code)
	assert.Contains(t, invalid.Body.String(), `"code":"unauthorized"`)
}

func TestAdminRoutesPermissionMatrix(t *testing.T) {
	admin := sampleUser()
	admin.Role = user.RoleAdmin
	admin.ID = 1
	viewer := sampleUser()
	viewer.ID = 2
	viewer.Role = user.RoleViewer

	users := fakeUsers{listFn: func(_ context.Context, actor user.Actor) ([]user.User, error) {
		require.Equal(t, admin.ID, actor.UserID)
		return []user.User{admin, viewer}, nil
	}}
	router, secret := testRouter(users)
	adminToken := tokenFor(t, secret, admin)
	viewerToken := tokenFor(t, secret, viewer)

	missing := perform(router, http.MethodGet, "/api/v1/admin/users", "", "")
	assert.Equal(t, http.StatusUnauthorized, missing.Code)

	forbidden := perform(router, http.MethodGet, "/api/v1/admin/users", "", "Bearer "+viewerToken)
	assert.Equal(t, http.StatusForbidden, forbidden.Code)
	assert.Contains(t, forbidden.Body.String(), `"code":"forbidden"`)

	ok := perform(router, http.MethodGet, "/api/v1/admin/users", "", "Bearer "+adminToken)
	require.Equal(t, http.StatusOK, ok.Code)
	assert.Contains(t, ok.Body.String(), `"email":"person@example.com"`)
	assert.NotContains(t, ok.Body.String(), "stored-password-hash")
}

func TestAdminRouteUsesFreshRoleFromDatabase(t *testing.T) {
	admin := sampleUser()
	admin.Role = user.RoleAdmin
	admin.ID = 7
	users := fakeUsers{
		byIDFn: func(context.Context, int64) (user.User, error) {
			return admin, nil
		},
		listFn: func(_ context.Context, actor user.Actor) ([]user.User, error) {
			require.Equal(t, user.RoleAdmin, actor.Role)
			return []user.User{admin}, nil
		},
	}
	secret := "handler-test-secret-never-return-this"
	tokens := auth.NewManager(secret, 15*time.Minute)
	h := auth.NewHandler(users, tokens)
	router := server.NewRouter(server.Dependencies{Auth: h, Tokens: tokens, Users: users})
	staleToken := tokenFor(t, secret, user.User{ID: admin.ID, Role: user.RoleViewer})

	res := perform(router, http.MethodGet, "/api/v1/admin/users", "", "Bearer "+staleToken)

	require.Equal(t, http.StatusOK, res.Code)
	assert.Contains(t, res.Body.String(), `"role":"admin"`)
}

func TestAdminLookupAndRoleUpdate(t *testing.T) {
	admin := sampleUser()
	admin.ID = 1
	admin.Role = user.RoleAdmin
	var gotRole user.Role
	users := fakeUsers{
		lookupFn: func(_ context.Context, actor user.Actor, id int64) (user.User, error) {
			require.Equal(t, admin.ID, actor.UserID)
			require.Equal(t, int64(9), id)
			u := sampleUser()
			u.ID = id
			return u, nil
		},
		assignRoleFn: func(_ context.Context, actor user.Actor, id int64, role user.Role) (user.User, error) {
			require.Equal(t, admin.ID, actor.UserID)
			require.Equal(t, int64(9), id)
			gotRole = role
			u := sampleUser()
			u.ID = id
			u.Role = role
			return u, nil
		},
	}
	router, secret := testRouter(users)
	token := tokenFor(t, secret, admin)

	found := perform(router, http.MethodGet, "/api/v1/admin/users/9", "", "Bearer "+token)
	require.Equal(t, http.StatusOK, found.Code)
	assert.Contains(t, found.Body.String(), `"id":9`)

	changed := perform(router, http.MethodPatch, "/api/v1/admin/users/9/role", `{"role":"dispatcher"}`, "Bearer "+token)
	require.Equal(t, http.StatusOK, changed.Code)
	assert.Equal(t, user.RoleDispatcher, gotRole)
	assert.Contains(t, changed.Body.String(), `"role":"dispatcher"`)
}

func TestAdminRoleErrors(t *testing.T) {
	admin := sampleUser()
	admin.ID = 1
	admin.Role = user.RoleAdmin
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "bad role", err: user.ErrInvalidRole, status: http.StatusBadRequest, code: "invalid_role"},
		{name: "missing user", err: user.ErrNotFound, status: http.StatusNotFound, code: "not_found"},
		{name: "last admin", err: user.ErrLastAdmin, status: http.StatusConflict, code: "last_admin"},
		{name: "service deny", err: user.ErrPermissionDenied, status: http.StatusForbidden, code: "forbidden"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := fakeUsers{assignRoleFn: func(context.Context, user.Actor, int64, user.Role) (user.User, error) {
				return user.User{}, tt.err
			}}
			router, secret := testRouter(users)
			token := tokenFor(t, secret, admin)
			res := perform(router, http.MethodPatch, "/api/v1/admin/users/2/role", `{"role":"viewer"}`, "Bearer "+token)
			assert.Equal(t, tt.status, res.Code)
			assert.Contains(t, res.Body.String(), `"code":"`+tt.code+`"`)
		})
	}
}

func TestAuthHandlerErrors(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{name: "duplicate", err: user.ErrEmailTaken, status: http.StatusConflict, code: "email_already_registered"},
		{name: "invalid email", err: user.ErrInvalidEmail, status: http.StatusBadRequest, code: "invalid_email"},
		{name: "unexpected", err: errors.New("database unavailable"), status: http.StatusInternalServerError, code: "internal_error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			users := fakeUsers{registerFn: func(context.Context, user.RegisterInput) (user.User, error) {
				return user.User{}, tt.err
			}}
			router, _ := testRouter(users)
			res := perform(router, http.MethodPost, "/api/v1/auth/register", `{"email":"p@example.com","password":"password1","full_name":"Pat"}`, "")
			assert.Equal(t, tt.status, res.Code)
			assert.Contains(t, res.Body.String(), `"code":"`+tt.code+`"`)
			assert.NotContains(t, res.Body.String(), "database unavailable")
		})
	}
}

func testRouter(users fakeUsers) (http.Handler, string) {
	secret := "handler-test-secret-never-return-this"
	tokens := auth.NewManager(secret, 15*time.Minute)
	h := auth.NewHandler(users, tokens)
	return server.NewRouter(server.Dependencies{Auth: h, Tokens: tokens}), secret
}

func sampleUser() user.User {
	return user.User{
		ID: 7, Email: "person@example.com", PasswordHash: "stored-password-hash",
		FullName: "Pat Smith", Role: user.RoleViewer,
		CreatedAt: time.Date(2023, 1, 10, 12, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2023, 1, 10, 12, 0, 0, 0, time.UTC),
	}
}

func tokenFor(t *testing.T, secret string, u user.User) string {
	t.Helper()
	raw, _, err := auth.NewManager(secret, 15*time.Minute).Issue(u)
	require.NoError(t, err)
	return raw
}

func perform(handler http.Handler, method, path, body, authorization string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if strings.TrimSpace(authorization) != "" {
		req.Header.Set("Authorization", authorization)
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

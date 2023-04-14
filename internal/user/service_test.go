package user

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	createFn     func(context.Context, string, string, string) (User, error)
	byEmailFn    func(context.Context, string) (User, error)
	byIDFn       func(context.Context, int64) (User, error)
	listFn       func(context.Context) ([]User, error)
	updateRoleFn func(context.Context, int64, Role) (User, error)
}

func (f fakeStore) Create(ctx context.Context, email, hash, name string) (User, error) {
	return f.createFn(ctx, email, hash, name)
}

func (f fakeStore) ByEmail(ctx context.Context, email string) (User, error) {
	return f.byEmailFn(ctx, email)
}

func (f fakeStore) ByID(ctx context.Context, id int64) (User, error) {
	return f.byIDFn(ctx, id)
}

func (f fakeStore) List(ctx context.Context) ([]User, error) {
	return f.listFn(ctx)
}

func (f fakeStore) UpdateRole(ctx context.Context, id int64, role Role) (User, error) {
	return f.updateRoleFn(ctx, id, role)
}

type fakePasswords struct {
	hashFn    func(string) (string, error)
	matchesFn func(string, string) bool
}

func (f fakePasswords) Hash(password string) (string, error) {
	return f.hashFn(password)
}

func (f fakePasswords) Matches(hash, password string) bool {
	return f.matchesFn(hash, password)
}

func TestRegisterNormalizesInputAndHashesPassword(t *testing.T) {
	var gotEmail, gotHash, gotName string
	repo := fakeStore{
		createFn: func(_ context.Context, email, hash, name string) (User, error) {
			gotEmail, gotHash, gotName = email, hash, name
			return User{ID: 1, Email: email, PasswordHash: hash, FullName: name, Role: RoleAdmin}, nil
		},
	}
	svc := NewService(repo)
	svc.pass = fakePasswords{
		hashFn: func(password string) (string, error) {
			assert.Equal(t, "strong-password", password)
			return "hashed-value", nil
		},
	}

	u, err := svc.Register(context.Background(), RegisterInput{
		Email: "  Person@Example.COM ", Password: "strong-password", FullName: "  Pat Smith  ",
	})

	require.NoError(t, err)
	assert.Equal(t, "person@example.com", gotEmail)
	assert.Equal(t, "hashed-value", gotHash)
	assert.NotEqual(t, "strong-password", gotHash)
	assert.Equal(t, "Pat Smith", gotName)
	assert.Equal(t, RoleAdmin, u.Role)
}

func TestRegisterValidation(t *testing.T) {
	tests := []struct {
		name string
		in   RegisterInput
		want error
	}{
		{name: "bad email", in: RegisterInput{Email: "nope", Password: "password1", FullName: "Pat"}, want: ErrInvalidEmail},
		{name: "short password", in: RegisterInput{Email: "p@example.com", Password: "short", FullName: "Pat"}, want: ErrInvalidPassword},
		{name: "bcrypt limit", in: RegisterInput{Email: "p@example.com", Password: string(make([]byte, 73)), FullName: "Pat"}, want: ErrInvalidPassword},
		{name: "blank name", in: RegisterInput{Email: "p@example.com", Password: "password1", FullName: "   "}, want: ErrInvalidName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewService(fakeStore{})
			_, err := svc.Register(context.Background(), tt.in)
			require.ErrorIs(t, err, tt.want)
		})
	}
}

func TestAuthenticate(t *testing.T) {
	stored := User{ID: 8, Email: "person@example.com", PasswordHash: "stored-hash", Role: RoleViewer}
	repo := fakeStore{
		byEmailFn: func(_ context.Context, email string) (User, error) {
			require.Equal(t, "person@example.com", email)
			return stored, nil
		},
	}
	svc := NewService(repo)
	svc.pass = fakePasswords{matchesFn: func(hash, password string) bool {
		return hash == "stored-hash" && password == "right-password"
	}}

	u, err := svc.Authenticate(context.Background(), " PERSON@EXAMPLE.COM ", "right-password")
	require.NoError(t, err)
	assert.Equal(t, stored, u)

	_, err = svc.Authenticate(context.Background(), "person@example.com", "wrong-password")
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestAuthenticateHidesMissingUser(t *testing.T) {
	repo := fakeStore{byEmailFn: func(context.Context, string) (User, error) {
		return User{}, ErrNotFound
	}}
	svc := NewService(repo)

	_, err := svc.Authenticate(context.Background(), "missing@example.com", "password1")
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestUserPasswordHashIsNotSerialized(t *testing.T) {
	u := User{ID: 1, Email: "p@example.com", PasswordHash: "do-not-return", Role: RoleViewer}
	raw, err := json.Marshal(u)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "do-not-return")
	assert.NotContains(t, string(raw), "password_hash")
}

func TestAdminOnlyUserActions(t *testing.T) {
	admin := User{ID: 1, Email: "admin@example.com", Role: RoleAdmin}
	viewer := User{ID: 2, Email: "viewer@example.com", Role: RoleViewer}
	repo := fakeStore{
		byIDFn: func(_ context.Context, id int64) (User, error) {
			if id == admin.ID {
				return admin, nil
			}
			if id == viewer.ID {
				return viewer, nil
			}
			return User{}, ErrNotFound
		},
		listFn: func(context.Context) ([]User, error) {
			return []User{admin, viewer}, nil
		},
		updateRoleFn: func(_ context.Context, id int64, role Role) (User, error) {
			require.Equal(t, viewer.ID, id)
			viewer.Role = role
			return viewer, nil
		},
	}
	svc := NewService(repo)

	arr, err := svc.List(context.Background(), Actor{UserID: admin.ID})
	require.NoError(t, err)
	require.Len(t, arr, 2)

	u, err := svc.Lookup(context.Background(), Actor{UserID: admin.ID}, viewer.ID)
	require.NoError(t, err)
	assert.Equal(t, viewer.Email, u.Email)

	changed, err := svc.AssignRole(context.Background(), Actor{UserID: admin.ID}, viewer.ID, RoleDispatcher)
	require.NoError(t, err)
	assert.Equal(t, RoleDispatcher, changed.Role)
}

func TestUserActionsDenyNonAdminsAndBadRoles(t *testing.T) {
	repo := fakeStore{byIDFn: func(_ context.Context, id int64) (User, error) {
		if id == 8 {
			return User{ID: 8, Role: RoleViewer}, nil
		}
		return User{}, ErrNotFound
	}}
	svc := NewService(repo)

	_, err := svc.List(context.Background(), Actor{UserID: 8})
	require.ErrorIs(t, err, ErrPermissionDenied)

	_, err = svc.AssignRole(context.Background(), Actor{UserID: 8}, 2, RoleAdmin)
	require.ErrorIs(t, err, ErrPermissionDenied)

	_, err = svc.AssignRole(context.Background(), Actor{UserID: 99}, 2, Role("boss"))
	require.ErrorIs(t, err, ErrPermissionDenied)

	adminRepo := fakeStore{byIDFn: func(context.Context, int64) (User, error) {
		return User{ID: 1, Role: RoleAdmin}, nil
	}}
	svc = NewService(adminRepo)
	_, err = svc.AssignRole(context.Background(), Actor{UserID: 1}, 2, Role("boss"))
	require.ErrorIs(t, err, ErrInvalidRole)
}

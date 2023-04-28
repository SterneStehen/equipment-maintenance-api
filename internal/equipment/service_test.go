package equipment

import (
	"context"
	"testing"

	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	createFn       func(context.Context, CreateInput) (Equipment, error)
	byIDFn         func(context.Context, int64) (Equipment, error)
	listFn         func(context.Context, ListFilter) ([]Equipment, error)
	updateFn       func(context.Context, int64, UpdateInput) (Equipment, error)
	decommissionFn func(context.Context, int64) (Equipment, error)
}

func (f fakeStore) Create(ctx context.Context, in CreateInput) (Equipment, error) {
	return f.createFn(ctx, in)
}

func (f fakeStore) ByID(ctx context.Context, id int64) (Equipment, error) {
	return f.byIDFn(ctx, id)
}

func (f fakeStore) List(ctx context.Context, flt ListFilter) ([]Equipment, error) {
	return f.listFn(ctx, flt)
}

func (f fakeStore) Update(ctx context.Context, id int64, in UpdateInput) (Equipment, error) {
	return f.updateFn(ctx, id, in)
}

func (f fakeStore) Decommission(ctx context.Context, id int64) (Equipment, error) {
	return f.decommissionFn(ctx, id)
}

func TestCreateNormalizesSerialAndRequiresWriter(t *testing.T) {
	var got CreateInput
	repo := fakeStore{createFn: func(_ context.Context, in CreateInput) (Equipment, error) {
		got = in
		return Equipment{ID: 3, SerialNumber: in.SerialNumber, Name: in.Name, Status: StatusActive}, nil
	}}
	svc := NewService(repo)

	x, err := svc.Create(context.Background(), user.Actor{Role: user.RoleDispatcher}, CreateInput{
		SerialNumber: "  pump-77  ", Name: "  Main pump  ", Model: "  MX ", Location: " floor 1 ",
	})
	require.NoError(t, err)
	assert.Equal(t, "PUMP-77", got.SerialNumber)
	assert.Equal(t, "Main pump", got.Name)
	assert.Equal(t, "MX", got.Model)
	assert.Equal(t, "floor 1", got.Location)
	assert.Equal(t, StatusActive, x.Status)

	_, err = svc.Create(context.Background(), user.Actor{Role: user.RoleViewer}, CreateInput{SerialNumber: "X", Name: "Thing"})
	require.ErrorIs(t, err, ErrPermissionDenied)
}

func TestEquipmentValidation(t *testing.T) {
	svc := NewService(fakeStore{})

	_, err := svc.Create(context.Background(), user.Actor{Role: user.RoleAdmin}, CreateInput{Name: "Pump"})
	require.ErrorIs(t, err, ErrInvalidSerial)

	_, err = svc.Create(context.Background(), user.Actor{Role: user.RoleAdmin}, CreateInput{SerialNumber: "A-1", Name: "  "})
	require.ErrorIs(t, err, ErrInvalidName)

	_, err = svc.Update(context.Background(), user.Actor{Role: user.RoleAdmin}, 1, UpdateInput{Name: "Pump", Status: StatusDecommissioned})
	require.ErrorIs(t, err, ErrInvalidStatus)

	_, err = svc.Update(context.Background(), user.Actor{Role: user.RoleAdmin}, 0, UpdateInput{Name: "Pump", Status: StatusActive})
	require.ErrorIs(t, err, ErrNotFound)
}

func TestListFilterAndPerms(t *testing.T) {
	var got ListFilter
	repo := fakeStore{
		listFn: func(_ context.Context, flt ListFilter) ([]Equipment, error) {
			got = flt
			return []Equipment{{ID: 1, SerialNumber: "A-1", Name: "Pump", Status: StatusActive}}, nil
		},
		byIDFn: func(_ context.Context, id int64) (Equipment, error) {
			return Equipment{ID: id, SerialNumber: "A-1", Name: "Pump", Status: StatusActive}, nil
		},
	}
	svc := NewService(repo)

	stuff, err := svc.List(context.Background(), user.Actor{Role: user.RoleTechnician}, ListFilter{
		Status: StatusActive, Query: "  pump ", Limit: 500, Offset: -10,
	})
	require.NoError(t, err)
	require.Len(t, stuff, 1)
	assert.Equal(t, StatusActive, got.Status)
	assert.Equal(t, "pump", got.Query)
	assert.Equal(t, maxLimit, got.Limit)
	assert.Equal(t, 0, got.Offset)

	_, err = svc.ByID(context.Background(), user.Actor{Role: user.RoleViewer}, 1)
	require.NoError(t, err)

	_, err = svc.List(context.Background(), user.Actor{Role: user.RoleViewer}, ListFilter{Status: Status("broken")})
	require.ErrorIs(t, err, ErrInvalidStatus)
}

func TestWritePermissionMatrix(t *testing.T) {
	tmp := fakeStore{
		updateFn: func(_ context.Context, id int64, in UpdateInput) (Equipment, error) {
			return Equipment{ID: id, Name: in.Name, Status: in.Status}, nil
		},
		decommissionFn: func(_ context.Context, id int64) (Equipment, error) {
			return Equipment{ID: id, Status: StatusDecommissioned}, nil
		},
	}
	svc := NewService(tmp)

	_, err := svc.Update(context.Background(), user.Actor{Role: user.RoleDispatcher}, 5, UpdateInput{Name: "Pump", Status: StatusMaintenance})
	require.NoError(t, err)

	_, err = svc.Update(context.Background(), user.Actor{Role: user.RoleTechnician}, 5, UpdateInput{Name: "Pump", Status: StatusMaintenance})
	require.ErrorIs(t, err, ErrPermissionDenied)

	_, err = svc.Decommission(context.Background(), user.Actor{Role: user.RoleDispatcher}, 5)
	require.ErrorIs(t, err, ErrPermissionDenied)

	_, err = svc.Decommission(context.Background(), user.Actor{Role: user.RoleAdmin}, 5)
	require.NoError(t, err)
}

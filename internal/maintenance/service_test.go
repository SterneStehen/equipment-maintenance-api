package maintenance

import (
	"context"
	"testing"

	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeMaintStore struct {
	listFn func(context.Context, ListFilter) ([]Record, error)
}

func (f fakeMaintStore) List(ctx context.Context, flt ListFilter) ([]Record, error) {
	return f.listFn(ctx, flt)
}

func TestListCleansFilter(t *testing.T) {
	var got ListFilter
	svc := NewService(fakeMaintStore{listFn: func(_ context.Context, flt ListFilter) ([]Record, error) {
		got = flt
		return []Record{{ID: 1, WorkOrderID: 4, EquipmentID: 9}}, nil
	}})

	arr, err := svc.List(context.Background(), user.Actor{Role: user.RoleTechnician}, ListFilter{
		WorkOrderID: 4,
		EquipmentID: 9,
		Limit:       999,
		Offset:      -5,
	})
	require.NoError(t, err)
	require.Len(t, arr, 1)
	assert.Equal(t, maxLimit, got.Limit)
	assert.Equal(t, 0, got.Offset)
	assert.Equal(t, int64(4), got.WorkOrderID)
}

func TestListUsesDefaultLimit(t *testing.T) {
	var got ListFilter
	svc := NewService(fakeMaintStore{listFn: func(_ context.Context, flt ListFilter) ([]Record, error) {
		got = flt
		return nil, nil
	}})

	_, err := svc.List(context.Background(), user.Actor{Role: user.RoleViewer}, ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, defaultLimit, got.Limit)
}

func TestListRejectsBadFilterAndRole(t *testing.T) {
	svc := NewService(fakeMaintStore{})

	_, err := svc.List(context.Background(), user.Actor{Role: user.Role("")}, ListFilter{})
	require.ErrorIs(t, err, ErrPermissionDenied)

	_, err = svc.List(context.Background(), user.Actor{Role: user.RoleViewer}, ListFilter{PerformedBy: -1})
	require.ErrorIs(t, err, ErrInvalidFilter)
}

package workorder

import (
	"context"
	"testing"

	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeStore struct {
	createFn     func(context.Context, CreateInput) (WorkOrder, error)
	byIDFn       func(context.Context, int64) (WorkOrder, error)
	listFn       func(context.Context, ListFilter) ([]WorkOrder, error)
	updateFn     func(context.Context, int64, UpdateInput) (WorkOrder, error)
	transitionFn func(context.Context, int64, TransitionInput) (WorkOrder, error)
	commentFn    func(context.Context, CommentInput) (Comment, error)
	commentsFn   func(context.Context, int64, int, int) ([]Comment, error)
}

func (f fakeStore) Create(ctx context.Context, in CreateInput) (WorkOrder, error) {
	return f.createFn(ctx, in)
}

func (f fakeStore) ByID(ctx context.Context, id int64) (WorkOrder, error) {
	return f.byIDFn(ctx, id)
}

func (f fakeStore) List(ctx context.Context, flt ListFilter) ([]WorkOrder, error) {
	return f.listFn(ctx, flt)
}

func (f fakeStore) Update(ctx context.Context, id int64, in UpdateInput) (WorkOrder, error) {
	return f.updateFn(ctx, id, in)
}

func (f fakeStore) Transition(ctx context.Context, id int64, in TransitionInput) (WorkOrder, error) {
	return f.transitionFn(ctx, id, in)
}

func (f fakeStore) CreateComment(ctx context.Context, in CommentInput) (Comment, error) {
	return f.commentFn(ctx, in)
}

func (f fakeStore) ListComments(ctx context.Context, id int64, limit, offset int) ([]Comment, error) {
	return f.commentsFn(ctx, id, limit, offset)
}

func TestCreateCleansInputAndSetsCreator(t *testing.T) {
	var got CreateInput
	techID := int64(7)
	tmp := fakeStore{createFn: func(_ context.Context, in CreateInput) (WorkOrder, error) {
		got = in
		return WorkOrder{ID: 2, EquipmentID: in.EquipmentID, Title: in.Title, Priority: in.Priority, AssignedTo: in.AssignedTo, CreatedBy: in.CreatedBy}, nil
	}}
	svc := NewService(tmp)

	wo, err := svc.Create(context.Background(), user.Actor{UserID: 3, Role: user.RoleDispatcher}, CreateInput{
		EquipmentID: 4, Title: "  replace belt ", Description: "  soon ", AssignedTo: &techID,
	})
	require.NoError(t, err)
	assert.Equal(t, "replace belt", got.Title)
	assert.Equal(t, "soon", got.Description)
	assert.Equal(t, PriorityMedium, got.Priority)
	assert.Equal(t, int64(3), got.CreatedBy)
	assert.Equal(t, &techID, wo.AssignedTo)
}

func TestWorkOrderValidation(t *testing.T) {
	svc := NewService(fakeStore{})

	_, err := svc.Create(context.Background(), user.Actor{Role: user.RoleAdmin}, CreateInput{Title: "Fix"})
	require.ErrorIs(t, err, ErrInvalidEquipment)

	_, err = svc.Create(context.Background(), user.Actor{Role: user.RoleAdmin}, CreateInput{EquipmentID: 1, Title: " "})
	require.ErrorIs(t, err, ErrInvalidTitle)

	_, err = svc.Create(context.Background(), user.Actor{Role: user.RoleAdmin}, CreateInput{EquipmentID: 1, Title: "Fix", Priority: Priority("bad")})
	require.ErrorIs(t, err, ErrInvalidPriority)

	_, err = svc.Update(context.Background(), user.Actor{Role: user.RoleAdmin}, 1, UpdateInput{Title: "Fix", Status: Status("done")})
	require.ErrorIs(t, err, ErrInvalidStatus)
}

func TestReadAndWritePermissions(t *testing.T) {
	tmp := fakeStore{
		byIDFn: func(_ context.Context, id int64) (WorkOrder, error) {
			return WorkOrder{ID: id, Title: "Fix", Status: StatusOpen}, nil
		},
		updateFn: func(_ context.Context, id int64, in UpdateInput) (WorkOrder, error) {
			return WorkOrder{ID: id, Title: in.Title, Status: in.Status, Priority: in.Priority}, nil
		},
	}
	svc := NewService(tmp)

	_, err := svc.ByID(context.Background(), user.Actor{Role: user.RoleViewer}, 1)
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), user.Actor{Role: user.RoleViewer}, CreateInput{EquipmentID: 1, Title: "Fix"})
	require.ErrorIs(t, err, ErrPermissionDenied)

	wo, err := svc.Update(context.Background(), user.Actor{Role: user.RoleDispatcher}, 2, UpdateInput{Title: "Fix", Status: StatusOpen, Priority: PriorityHigh})
	require.NoError(t, err)
	assert.Equal(t, StatusOpen, wo.Status)
}

func TestListFilterDefaults(t *testing.T) {
	var got ListFilter
	tmp := fakeStore{listFn: func(_ context.Context, flt ListFilter) ([]WorkOrder, error) {
		got = flt
		return []WorkOrder{{ID: 1, Title: "Fix"}}, nil
	}}
	svc := NewService(tmp)

	arr, err := svc.List(context.Background(), user.Actor{Role: user.RoleTechnician}, ListFilter{
		Status: StatusOpen, Priority: PriorityUrgent, Query: "  pump ", Limit: 900, Offset: -1,
	})
	require.NoError(t, err)
	require.Len(t, arr, 1)
	assert.Equal(t, maxLimit, got.Limit)
	assert.Equal(t, 0, got.Offset)
	assert.Equal(t, "pump", got.Query)
}

func TestListRejectsBadFilters(t *testing.T) {
	svc := NewService(fakeStore{})

	_, err := svc.List(context.Background(), user.Actor{Role: user.RoleViewer}, ListFilter{Status: Status("later")})
	require.ErrorIs(t, err, ErrInvalidStatus)

	_, err = svc.List(context.Background(), user.Actor{Role: user.RoleViewer}, ListFilter{Priority: Priority("mega")})
	require.ErrorIs(t, err, ErrInvalidPriority)

	_, err = svc.List(context.Background(), user.Actor{Role: user.RoleViewer}, ListFilter{EquipmentID: -3})
	require.ErrorIs(t, err, ErrInvalidEquipment)
}

func TestTransitionHelpers(t *testing.T) {
	var got TransitionInput
	tmp := fakeStore{transitionFn: func(_ context.Context, id int64, in TransitionInput) (WorkOrder, error) {
		require.Equal(t, int64(7), id)
		got = in
		return WorkOrder{ID: id, Status: in.ToStatus}, nil
	}}
	svc := NewService(tmp)

	wo, err := svc.Start(context.Background(), user.Actor{UserID: 4, Role: user.RoleTechnician}, 7, "  grabbing it ")
	require.NoError(t, err)
	assert.Equal(t, StatusInProgress, wo.Status)
	assert.Equal(t, int64(4), got.ActorID)
	assert.Equal(t, user.RoleTechnician, got.ActorRole)
	assert.Equal(t, "grabbing it", got.Note)

	wo, err = svc.Cancel(context.Background(), user.Actor{UserID: 2, Role: user.RoleDispatcher}, 7, "")
	require.NoError(t, err)
	assert.Equal(t, StatusCanceled, wo.Status)
}

func TestComments(t *testing.T) {
	var got CommentInput
	tmp := fakeStore{
		commentFn: func(_ context.Context, in CommentInput) (Comment, error) {
			got = in
			return Comment{ID: 1, WorkOrderID: in.WorkOrderID, AuthorID: in.AuthorID, Body: in.Body}, nil
		},
		commentsFn: func(_ context.Context, id int64, limit, offset int) ([]Comment, error) {
			require.Equal(t, int64(8), id)
			require.Equal(t, maxLimit, limit)
			require.Equal(t, 0, offset)
			return []Comment{{ID: 1, Body: "ok"}}, nil
		},
	}
	svc := NewService(tmp)

	c, err := svc.AddComment(context.Background(), user.Actor{UserID: 3, Role: user.RoleViewer}, 8, "  nice fix ")
	require.NoError(t, err)
	assert.Equal(t, "nice fix", c.Body)
	assert.Equal(t, int64(3), got.AuthorID)

	arr, err := svc.ListComments(context.Background(), user.Actor{Role: user.RoleViewer}, 8, 500, -3)
	require.NoError(t, err)
	require.Len(t, arr, 1)

	_, err = svc.AddComment(context.Background(), user.Actor{Role: user.RoleViewer}, 8, " ")
	require.ErrorIs(t, err, ErrInvalidComment)
}

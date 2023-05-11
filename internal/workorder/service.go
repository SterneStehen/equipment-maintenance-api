package workorder

import (
	"context"
	"strings"

	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type store interface {
	Create(ctx context.Context, in CreateInput) (WorkOrder, error)
	ByID(ctx context.Context, id int64) (WorkOrder, error)
	List(ctx context.Context, f ListFilter) ([]WorkOrder, error)
	Update(ctx context.Context, id int64, in UpdateInput) (WorkOrder, error)
}

type Service struct {
	repo store
}

type CreateInput struct {
	EquipmentID int64
	Title       string
	Description string
	Priority    Priority
	AssignedTo  *int64
	CreatedBy   int64
}

type UpdateInput struct {
	Title       string
	Description string
	Status      Status
	Priority    Priority
	AssignedTo  *int64
}

func NewService(repo store) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, actor user.Actor, in CreateInput) (WorkOrder, error) {
	if !canWrite(actor.Role) {
		return WorkOrder{}, ErrPermissionDenied
	}
	x, err := cleanCreate(in)
	if err != nil {
		return WorkOrder{}, err
	}
	x.CreatedBy = actor.UserID
	return s.repo.Create(ctx, x)
}

func (s *Service) ByID(ctx context.Context, actor user.Actor, id int64) (WorkOrder, error) {
	if !canRead(actor.Role) {
		return WorkOrder{}, ErrPermissionDenied
	}
	if id < 1 {
		return WorkOrder{}, ErrNotFound
	}
	return s.repo.ByID(ctx, id)
}

func (s *Service) List(ctx context.Context, actor user.Actor, f ListFilter) ([]WorkOrder, error) {
	if !canRead(actor.Role) {
		return nil, ErrPermissionDenied
	}
	flt, err := cleanFilter(f)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, flt)
}

func (s *Service) Update(ctx context.Context, actor user.Actor, id int64, in UpdateInput) (WorkOrder, error) {
	if !canWrite(actor.Role) {
		return WorkOrder{}, ErrPermissionDenied
	}
	if id < 1 {
		return WorkOrder{}, ErrNotFound
	}
	x, err := cleanUpdate(in)
	if err != nil {
		return WorkOrder{}, err
	}
	return s.repo.Update(ctx, id, x)
}

func cleanCreate(in CreateInput) (CreateInput, error) {
	if in.EquipmentID < 1 {
		return CreateInput{}, ErrInvalidEquipment
	}
	title := strings.TrimSpace(in.Title)
	if title == "" || len(title) > 220 {
		return CreateInput{}, ErrInvalidTitle
	}
	pr := in.Priority
	if pr == "" {
		pr = PriorityMedium
	}
	if !pr.Valid() {
		return CreateInput{}, ErrInvalidPriority
	}
	return CreateInput{
		EquipmentID: in.EquipmentID,
		Title:       title,
		Description: trimTo(in.Description, 4000),
		Priority:    pr,
		AssignedTo:  in.AssignedTo,
		CreatedBy:   in.CreatedBy,
	}, nil
}

func cleanUpdate(in UpdateInput) (UpdateInput, error) {
	title := strings.TrimSpace(in.Title)
	if title == "" || len(title) > 220 {
		return UpdateInput{}, ErrInvalidTitle
	}
	st := in.Status
	if st == "" {
		st = StatusOpen
	}
	if !st.Valid() {
		return UpdateInput{}, ErrInvalidStatus
	}
	pr := in.Priority
	if pr == "" {
		pr = PriorityMedium
	}
	if !pr.Valid() {
		return UpdateInput{}, ErrInvalidPriority
	}
	return UpdateInput{
		Title:       title,
		Description: trimTo(in.Description, 4000),
		Status:      st,
		Priority:    pr,
		AssignedTo:  in.AssignedTo,
	}, nil
}

func cleanFilter(f ListFilter) (ListFilter, error) {
	if f.Status != "" && !f.Status.Valid() {
		return ListFilter{}, ErrInvalidStatus
	}
	if f.Priority != "" && !f.Priority.Valid() {
		return ListFilter{}, ErrInvalidPriority
	}
	if f.EquipmentID < 0 || f.AssignedTo < 0 {
		return ListFilter{}, ErrInvalidEquipment
	}
	if f.Limit <= 0 {
		f.Limit = defaultLimit
	}
	if f.Limit > maxLimit {
		f.Limit = maxLimit
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	f.Query = trimTo(f.Query, 120)
	return f, nil
}

func trimTo(raw string, n int) string {
	v := strings.TrimSpace(raw)
	if len(v) > n {
		return v[:n]
	}
	return v
}

func canRead(role user.Role) bool {
	return role == user.RoleAdmin || role == user.RoleDispatcher || role == user.RoleTechnician || role == user.RoleViewer
}

func canWrite(role user.Role) bool {
	return role == user.RoleAdmin || role == user.RoleDispatcher
}

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
	Transition(ctx context.Context, id int64, in TransitionInput) (WorkOrder, error)
	CreateComment(ctx context.Context, in CommentInput) (Comment, error)
	ListComments(ctx context.Context, workOrderID int64, limit, offset int) ([]Comment, error)
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

type TransitionInput struct {
	ActorID   int64
	ActorRole user.Role
	ToStatus  Status
	Note      string
}

type CommentInput struct {
	WorkOrderID int64
	AuthorID    int64
	Body        string
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

func (s *Service) Start(ctx context.Context, actor user.Actor, id int64, note string) (WorkOrder, error) {
	return s.move(ctx, actor, id, StatusInProgress, note)
}

func (s *Service) Complete(ctx context.Context, actor user.Actor, id int64, note string) (WorkOrder, error) {
	return s.move(ctx, actor, id, StatusCompleted, note)
}

func (s *Service) Close(ctx context.Context, actor user.Actor, id int64, note string) (WorkOrder, error) {
	return s.move(ctx, actor, id, StatusClosed, note)
}

func (s *Service) Cancel(ctx context.Context, actor user.Actor, id int64, note string) (WorkOrder, error) {
	return s.move(ctx, actor, id, StatusCanceled, note)
}

func (s *Service) AddComment(ctx context.Context, actor user.Actor, id int64, body string) (Comment, error) {
	if !canRead(actor.Role) {
		return Comment{}, ErrPermissionDenied
	}
	if id < 1 {
		return Comment{}, ErrNotFound
	}
	txt := trimTo(body, 2000)
	if txt == "" {
		return Comment{}, ErrInvalidComment
	}
	return s.repo.CreateComment(ctx, CommentInput{WorkOrderID: id, AuthorID: actor.UserID, Body: txt})
}

func (s *Service) ListComments(ctx context.Context, actor user.Actor, id int64, limit, offset int) ([]Comment, error) {
	if !canRead(actor.Role) {
		return nil, ErrPermissionDenied
	}
	if id < 1 {
		return nil, ErrNotFound
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	if limit > maxLimit {
		limit = maxLimit
	}
	if offset < 0 {
		offset = 0
	}
	return s.repo.ListComments(ctx, id, limit, offset)
}

func (s *Service) move(ctx context.Context, actor user.Actor, id int64, to Status, note string) (WorkOrder, error) {
	if !canRead(actor.Role) {
		return WorkOrder{}, ErrPermissionDenied
	}
	if id < 1 {
		return WorkOrder{}, ErrNotFound
	}
	return s.repo.Transition(ctx, id, TransitionInput{
		ActorID: actor.UserID, ActorRole: actor.Role, ToStatus: to, Note: trimTo(note, 1000),
	})
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
	if st != StatusOpen {
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

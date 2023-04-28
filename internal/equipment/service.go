package equipment

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
	Create(ctx context.Context, in CreateInput) (Equipment, error)
	ByID(ctx context.Context, id int64) (Equipment, error)
	List(ctx context.Context, f ListFilter) ([]Equipment, error)
	Update(ctx context.Context, id int64, in UpdateInput) (Equipment, error)
	Decommission(ctx context.Context, id int64) (Equipment, error)
}

type Service struct {
	repo store
}

type CreateInput struct {
	SerialNumber string
	Name         string
	Model        string
	Location     string
	Notes        string
}

type UpdateInput struct {
	Name     string
	Model    string
	Location string
	Status   Status
	Notes    string
}

func NewService(repo store) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, actor user.Actor, in CreateInput) (Equipment, error) {
	if !canEditGear(actor.Role) {
		return Equipment{}, ErrPermissionDenied
	}

	clean, err := cleanCreate(in)
	if err != nil {
		return Equipment{}, err
	}
	return s.repo.Create(ctx, clean)
}

func (s *Service) ByID(ctx context.Context, actor user.Actor, id int64) (Equipment, error) {
	if !canSeeGear(actor.Role) {
		return Equipment{}, ErrPermissionDenied
	}
	if id < 1 {
		return Equipment{}, ErrNotFound
	}
	return s.repo.ByID(ctx, id)
}

func (s *Service) List(ctx context.Context, actor user.Actor, f ListFilter) ([]Equipment, error) {
	if !canSeeGear(actor.Role) {
		return nil, ErrPermissionDenied
	}

	flt, err := cleanFilter(f)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, flt)
}

func (s *Service) Update(ctx context.Context, actor user.Actor, id int64, in UpdateInput) (Equipment, error) {
	if !canEditGear(actor.Role) {
		return Equipment{}, ErrPermissionDenied
	}
	if id < 1 {
		return Equipment{}, ErrNotFound
	}
	x, err := cleanUpdate(in)
	if err != nil {
		return Equipment{}, err
	}
	return s.repo.Update(ctx, id, x)
}

func (s *Service) Decommission(ctx context.Context, actor user.Actor, id int64) (Equipment, error) {
	if actor.Role != user.RoleAdmin {
		return Equipment{}, ErrPermissionDenied
	}
	if id < 1 {
		return Equipment{}, ErrNotFound
	}
	return s.repo.Decommission(ctx, id)
}

func cleanCreate(in CreateInput) (CreateInput, error) {
	serial := normSerial(in.SerialNumber)
	if serial == "" || len(serial) > 120 {
		return CreateInput{}, ErrInvalidSerial
	}
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 200 {
		return CreateInput{}, ErrInvalidName
	}
	return CreateInput{
		SerialNumber: serial,
		Name:         name,
		Model:        trimTo(in.Model, 160),
		Location:     trimTo(in.Location, 160),
		Notes:        trimTo(in.Notes, 2000),
	}, nil
}

func cleanUpdate(in UpdateInput) (UpdateInput, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" || len(name) > 200 {
		return UpdateInput{}, ErrInvalidName
	}
	st := in.Status
	if st == "" {
		st = StatusActive
	}
	if !st.Valid() || st == StatusDecommissioned {
		return UpdateInput{}, ErrInvalidStatus
	}
	return UpdateInput{
		Name:     name,
		Model:    trimTo(in.Model, 160),
		Location: trimTo(in.Location, 160),
		Status:   st,
		Notes:    trimTo(in.Notes, 2000),
	}, nil
}

func cleanFilter(f ListFilter) (ListFilter, error) {
	if f.Status != "" && !f.Status.Valid() {
		return ListFilter{}, ErrInvalidStatus
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
	q := trimTo(f.Query, 120)
	f.Query = q
	return f, nil
}

func normSerial(raw string) string {
	return strings.ToUpper(strings.TrimSpace(raw))
}

func trimTo(raw string, n int) string {
	v := strings.TrimSpace(raw)
	if len(v) > n {
		return v[:n]
	}
	return v
}

func canSeeGear(role user.Role) bool {
	return role == user.RoleAdmin || role == user.RoleDispatcher || role == user.RoleTechnician || role == user.RoleViewer
}

func canEditGear(role user.Role) bool {
	return role == user.RoleAdmin || role == user.RoleDispatcher
}

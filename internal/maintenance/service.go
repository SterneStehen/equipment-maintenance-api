package maintenance

import (
	"context"

	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
)

const (
	defaultLimit = 20
	maxLimit     = 100
)

type store interface {
	List(ctx context.Context, f ListFilter) ([]Record, error)
}

type Service struct {
	repo store
}

func NewService(repo store) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, actor user.Actor, f ListFilter) ([]Record, error) {
	if !canRead(actor.Role) {
		return nil, ErrPermissionDenied
	}
	clean, err := cleanFilter(f)
	if err != nil {
		return nil, err
	}
	return s.repo.List(ctx, clean)
}

func cleanFilter(f ListFilter) (ListFilter, error) {
	if f.WorkOrderID < 0 || f.EquipmentID < 0 || f.PerformedBy < 0 {
		return ListFilter{}, ErrInvalidFilter
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
	return f, nil
}

func canRead(role user.Role) bool {
	return role == user.RoleAdmin || role == user.RoleDispatcher || role == user.RoleTechnician || role == user.RoleViewer
}

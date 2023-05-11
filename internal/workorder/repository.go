package workorder

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/SterneStehen/equipment-maintenance-api/internal/equipment"
	"github.com/SterneStehen/equipment-maintenance-api/internal/user"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (WorkOrder, error) {
	if err := r.checkEquipment(ctx, in.EquipmentID); err != nil {
		return WorkOrder{}, err
	}
	if err := r.checkAssignee(ctx, in.AssignedTo); err != nil {
		return WorkOrder{}, err
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO work_orders (equipment_id, title, description, status, priority, assigned_to, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, equipment_id, title, description, status, priority, assigned_to, created_by, created_at, updated_at, completed_at
	`, in.EquipmentID, in.Title, in.Description, StatusOpen, in.Priority, in.AssignedTo, in.CreatedBy)
	wo, err := scanWO(row)
	if err != nil {
		return WorkOrder{}, fmt.Errorf("insert work order: %w", err)
	}
	return wo, nil
}

func (r *Repository) ByID(ctx context.Context, id int64) (WorkOrder, error) {
	wo, err := scanWO(r.pool.QueryRow(ctx, `
		SELECT id, equipment_id, title, description, status, priority, assigned_to, created_by, created_at, updated_at, completed_at
		FROM work_orders
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkOrder{}, ErrNotFound
	}
	if err != nil {
		return WorkOrder{}, fmt.Errorf("find work order: %w", err)
	}
	return wo, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]WorkOrder, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, equipment_id, title, description, status, priority, assigned_to, created_by, created_at, updated_at, completed_at
		FROM work_orders
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR priority = $2)
		  AND ($3 = 0 OR equipment_id = $3)
		  AND ($4 = 0 OR assigned_to = $4)
		  AND ($5 = '' OR title ILIKE '%' || $5 || '%' OR description ILIKE '%' || $5 || '%')
		ORDER BY id DESC
		LIMIT $6 OFFSET $7
	`, f.Status, f.Priority, f.EquipmentID, f.AssignedTo, f.Query, f.Limit, f.Offset)
	if err != nil {
		return nil, fmt.Errorf("list work orders: %w", err)
	}
	defer rows.Close()

	var arr []WorkOrder
	for rows.Next() {
		x, err := scanWO(rows)
		if err != nil {
			return nil, fmt.Errorf("scan work order: %w", err)
		}
		arr = append(arr, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read work orders: %w", err)
	}
	return arr, nil
}

func (r *Repository) Update(ctx context.Context, id int64, in UpdateInput) (WorkOrder, error) {
	if err := r.checkAssignee(ctx, in.AssignedTo); err != nil {
		return WorkOrder{}, err
	}

	var eqID int64
	err := r.pool.QueryRow(ctx, "SELECT equipment_id FROM work_orders WHERE id = $1", id).Scan(&eqID)
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkOrder{}, ErrNotFound
	}
	if err != nil {
		return WorkOrder{}, fmt.Errorf("read work order equipment: %w", err)
	}
	if err := r.checkEquipment(ctx, eqID); err != nil {
		return WorkOrder{}, err
	}

	wo, err := scanWO(r.pool.QueryRow(ctx, `
		UPDATE work_orders
		SET title = $2, description = $3, status = $4, priority = $5, assigned_to = $6,
		    updated_at = NOW(),
		    completed_at = CASE WHEN $4 = 'completed' THEN COALESCE(completed_at, NOW()) ELSE NULL END
		WHERE id = $1
		RETURNING id, equipment_id, title, description, status, priority, assigned_to, created_by, created_at, updated_at, completed_at
	`, id, in.Title, in.Description, in.Status, in.Priority, in.AssignedTo))
	if errors.Is(err, pgx.ErrNoRows) {
		return WorkOrder{}, ErrNotFound
	}
	if err != nil {
		return WorkOrder{}, fmt.Errorf("update work order: %w", err)
	}
	return wo, nil
}

func (r *Repository) checkEquipment(ctx context.Context, id int64) error {
	var st equipment.Status
	err := r.pool.QueryRow(ctx, "SELECT status FROM equipment WHERE id = $1", id).Scan(&st)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrEquipmentNotFound
	}
	if err != nil {
		return fmt.Errorf("check equipment: %w", err)
	}
	if st == equipment.StatusDecommissioned {
		return ErrEquipmentDecommissioned
	}
	return nil
}

func (r *Repository) checkAssignee(ctx context.Context, id *int64) error {
	if id == nil {
		return nil
	}
	var role user.Role
	err := r.pool.QueryRow(ctx, "SELECT role FROM users WHERE id = $1", *id).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrAssigneeNotFound
	}
	if err != nil {
		return fmt.Errorf("check assignee: %w", err)
	}
	if role != user.RoleTechnician {
		return ErrAssigneeNotTechnician
	}
	return nil
}

type rowish interface {
	Scan(dest ...interface{}) error
}

func scanWO(row rowish) (WorkOrder, error) {
	var x WorkOrder
	err := row.Scan(&x.ID, &x.EquipmentID, &x.Title, &x.Description, &x.Status, &x.Priority, &x.AssignedTo, &x.CreatedBy, &x.CreatedAt, &x.UpdatedAt, &x.CompletedAt)
	return x, err
}

func likeQ(raw string) string {
	return strings.TrimSpace(raw)
}

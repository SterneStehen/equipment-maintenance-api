package maintenance

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]Record, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, work_order_id, equipment_id, performed_by, notes, created_at
		FROM maintenance_records
		WHERE ($1 = 0 OR work_order_id = $1)
		  AND ($2 = 0 OR equipment_id = $2)
		  AND ($3 = 0 OR performed_by = $3)
		ORDER BY id DESC
		LIMIT $4 OFFSET $5
	`, f.WorkOrderID, f.EquipmentID, f.PerformedBy, f.Limit, f.Offset)
	if err != nil {
		return nil, fmt.Errorf("list maintenance records: %w", err)
	}
	defer rows.Close()

	var arr []Record
	for rows.Next() {
		var rec Record
		if err := rows.Scan(&rec.ID, &rec.WorkOrderID, &rec.EquipmentID, &rec.PerformedBy, &rec.Notes, &rec.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan maintenance record: %w", err)
		}
		arr = append(arr, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read maintenance records: %w", err)
	}
	return arr, nil
}

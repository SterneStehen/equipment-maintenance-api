package equipment

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, in CreateInput) (Equipment, error) {
	row := r.pool.QueryRow(ctx, `
		INSERT INTO equipment (serial_number, name, model, location, status, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, serial_number, name, model, location, status, notes, created_at, updated_at, decommissioned_at
	`, in.SerialNumber, in.Name, in.Model, in.Location, StatusActive, in.Notes)
	x, err := scanOne(row)
	if err != nil {
		if dupSerial(err) {
			return Equipment{}, ErrSerialTaken
		}
		return Equipment{}, fmt.Errorf("insert equipment: %w", err)
	}
	return x, nil
}

func (r *Repository) ByID(ctx context.Context, id int64) (Equipment, error) {
	x, err := scanOne(r.pool.QueryRow(ctx, `
		SELECT id, serial_number, name, model, location, status, notes, created_at, updated_at, decommissioned_at
		FROM equipment
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return Equipment{}, ErrNotFound
	}
	if err != nil {
		return Equipment{}, fmt.Errorf("find equipment: %w", err)
	}
	return x, nil
}

func (r *Repository) List(ctx context.Context, f ListFilter) ([]Equipment, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, serial_number, name, model, location, status, notes, created_at, updated_at, decommissioned_at
		FROM equipment
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR serial_number ILIKE '%' || $2 || '%' OR name ILIKE '%' || $2 || '%' OR location ILIKE '%' || $2 || '%')
		ORDER BY id
		LIMIT $3 OFFSET $4
	`, f.Status, f.Query, f.Limit, f.Offset)
	if err != nil {
		return nil, fmt.Errorf("list equipment: %w", err)
	}
	defer rows.Close()

	var arr []Equipment
	for rows.Next() {
		x, err := scanOne(rows)
		if err != nil {
			return nil, fmt.Errorf("scan equipment: %w", err)
		}
		arr = append(arr, x)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read equipment rows: %w", err)
	}
	return arr, nil
}

func (r *Repository) Update(ctx context.Context, id int64, in UpdateInput) (Equipment, error) {
	x, err := scanOne(r.pool.QueryRow(ctx, `
		UPDATE equipment
		SET name = $2, model = $3, location = $4, status = $5, notes = $6, updated_at = NOW()
		WHERE id = $1 AND status <> 'decommissioned'
		RETURNING id, serial_number, name, model, location, status, notes, created_at, updated_at, decommissioned_at
	`, id, in.Name, in.Model, in.Location, in.Status, in.Notes))
	if errors.Is(err, pgx.ErrNoRows) {
		return Equipment{}, ErrNotFound
	}
	if err != nil {
		return Equipment{}, fmt.Errorf("update equipment: %w", err)
	}
	return x, nil
}

func (r *Repository) Decommission(ctx context.Context, id int64) (Equipment, error) {
	x, err := scanOne(r.pool.QueryRow(ctx, `
		UPDATE equipment
		SET status = 'decommissioned', decommissioned_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status <> 'decommissioned'
		RETURNING id, serial_number, name, model, location, status, notes, created_at, updated_at, decommissioned_at
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		old, err2 := r.ByID(ctx, id)
		if errors.Is(err2, ErrNotFound) {
			return Equipment{}, ErrNotFound
		}
		if err2 != nil {
			return Equipment{}, err2
		}
		if old.Status == StatusDecommissioned {
			return Equipment{}, ErrAlreadyRetired
		}
		return Equipment{}, ErrNotFound
	}
	if err != nil {
		return Equipment{}, fmt.Errorf("decommission equipment: %w", err)
	}
	return x, nil
}

type rowish interface {
	Scan(dest ...interface{}) error
}

func scanOne(row rowish) (Equipment, error) {
	var x Equipment
	err := row.Scan(&x.ID, &x.SerialNumber, &x.Name, &x.Model, &x.Location, &x.Status, &x.Notes, &x.CreatedAt, &x.UpdatedAt, &x.DecommissionedAt)
	return x, err
}

func dupSerial(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

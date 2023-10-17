package audit

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Record(ctx context.Context, in EventInput) error {
	action := strings.TrimSpace(in.Action)
	target := strings.TrimSpace(in.Target)
	if action == "" || target == "" {
		return nil
	}

	var actor *int64
	if in.ActorID > 0 {
		actor = &in.ActorID
	}
	var targetID *int64
	if in.TargetID > 0 {
		targetID = &in.TargetID
	}
	if _, err := r.pool.Exec(ctx, `
		INSERT INTO audit_events (actor_id, action, target_type, target_id, details)
		VALUES ($1, $2, $3, $4, $5)
	`, actor, action, target, targetID, strings.TrimSpace(in.Details)); err != nil {
		return fmt.Errorf("record audit event: %w", err)
	}
	return nil
}

package audit

import (
	"context"
	"time"
)

type Event struct {
	ID        int64     `json:"id"`
	ActorID   *int64    `json:"actor_id,omitempty"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	TargetID  *int64    `json:"target_id,omitempty"`
	Details   string    `json:"details"`
	CreatedAt time.Time `json:"created_at"`
}

type EventInput struct {
	ActorID  int64
	Action   string
	Target   string
	TargetID int64
	Details  string
}

type Recorder interface {
	Record(ctx context.Context, in EventInput) error
}

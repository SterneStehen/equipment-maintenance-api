package workorder

import (
	"errors"
	"time"
)

type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in_progress"
	StatusCompleted  Status = "completed"
	StatusCanceled   Status = "canceled"
)

func (s Status) Valid() bool {
	return s == StatusOpen || s == StatusInProgress || s == StatusCompleted || s == StatusCanceled
}

type Priority string

const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)





	func (p Priority) Valid() bool {
		return p == PriorityLow || p == PriorityMedium || p == PriorityHigh || p == PriorityUrgent
	}

	type WorkOrder struct {
		ID          int64      `json:"id"`
		EquipmentID int64      `json:"equipment_id"`
		Title       string     `json:"title"`
		Description string     `json:"description"`
		Status      Status     `json:"status"`
		Priority    Priority   `json:"priority"`
		AssignedTo  *int64     `json:"assigned_to,omitempty"`
		CreatedBy   int64      `json:"created_by"`
		CreatedAt   time.Time  `json:"created_at"`
		UpdatedAt   time.Time  `json:"updated_at"`
		CompletedAt *time.Time `json:"completed_at,omitempty"`
	}

	type ListFilter struct {
		Status      Status
		Priority    Priority
		EquipmentID int64
		AssignedTo  int64
		Query       string
		Limit       int
		Offset      int
	}

	var (
		ErrNotFound                = errors.New("work order not found")
		ErrInvalidTitle            = errors.New("invalid work order title")
		ErrInvalidStatus           = errors.New("invalid work order status")
		ErrInvalidPriority         = errors.New("invalid work order priority")
		ErrInvalidEquipment        = errors.New("invalid equipment")
		ErrEquipmentNotFound       = errors.New("equipment not found")
		ErrEquipmentDecommissioned = errors.New("equipment is decommissioned")
		ErrAssigneeNotFound        = errors.New("assignee not found")
		ErrAssigneeNotTechnician   = errors.New("assignee is not a technician")
		ErrPermissionDenied        = errors.New("permission denied")
	)

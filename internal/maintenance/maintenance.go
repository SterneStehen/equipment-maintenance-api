package maintenance

import (
	"errors"
	"time"
)

type Record struct {
	ID          int64     `json:"id"`
	WorkOrderID int64     `json:"work_order_id"`
	EquipmentID int64     `json:"equipment_id"`
	PerformedBy int64     `json:"performed_by"`
	Notes       string    `json:"notes"`
	CreatedAt   time.Time `json:"created_at"`
}

type ListFilter struct {
	WorkOrderID int64
	EquipmentID int64
	PerformedBy int64
	Limit       int
	Offset      int
}

var (
	ErrInvalidFilter    = errors.New("invalid maintenance filter")
	ErrPermissionDenied = errors.New("permission denied")
)

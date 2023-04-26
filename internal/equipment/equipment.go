package equipment

import (
	"errors"
	"time"
)

type Status string

const (
	StatusActive         Status = "active"
	StatusMaintenance    Status = "maintenance"
	StatusDecommissioned Status = "decommissioned"
)

func (s Status) Valid() bool {
	return s == StatusActive || s == StatusMaintenance || s == StatusDecommissioned
}

type Equipment struct {
	ID               int64      `json:"id"`
	SerialNumber     string     `json:"serial_number"`
	Name             string     `json:"name"`
	Model            string     `json:"model"`
	Location         string     `json:"location"`
	Status           Status     `json:"status"`
	Notes            string     `json:"notes"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DecommissionedAt *time.Time `json:"decommissioned_at,omitempty"`
}

type ListFilter struct {
	Status Status
	Query  string
	Limit  int
	Offset int
}

var (
	ErrSerialTaken      = errors.New("equipment serial number already exists")
	ErrNotFound         = errors.New("equipment not found")
	ErrInvalidSerial    = errors.New("invalid serial number")
	ErrInvalidName      = errors.New("invalid equipment name")
	ErrInvalidStatus    = errors.New("invalid equipment status")
	ErrPermissionDenied = errors.New("permission denied")
	ErrAlreadyRetired   = errors.New("equipment already decommissioned")
)

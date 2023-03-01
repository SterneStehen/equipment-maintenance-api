package user

import (
	"errors"
	"time"
)

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleDispatcher Role = "dispatcher"
	RoleTechnician Role = "technician"
	RoleViewer     Role = "viewer"
)

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleDispatcher || r == RoleTechnician || r == RoleViewer
}

type User struct {
	ID           int64     `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	FullName     string    `json:"full_name"`
	Role         Role      `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrNotFound           = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidEmail       = errors.New("invalid email")
	ErrInvalidPassword    = errors.New("invalid password")
	ErrInvalidName        = errors.New("invalid full name")
)

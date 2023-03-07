package user

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

type store interface {
	Create(ctx context.Context, email, passwordHash, fullName string) (User, error)
	ByEmail(ctx context.Context, email string) (User, error)
	ByID(ctx context.Context, id int64) (User, error)
}

type passOps interface {
	Hash(password string) (string, error)
	Matches(hash, password string) bool
}

type bcryptPass struct {
	cost int
}

func (b bcryptPass) Hash(password string) (string, error) {
	v, err := bcrypt.GenerateFromPassword([]byte(password), b.cost)
	return string(v), err
}

func (b bcryptPass) Matches(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

type Service struct {
	repo store
	pass passOps
}

type RegisterInput struct {
	Email    string
	Password string
	FullName string
}

func NewService(repo store) *Service {
	return &Service{repo: repo, pass: bcryptPass{cost: bcrypt.DefaultCost}}
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (User, error) {
	email, err := cleanEmail(in.Email)
	if err != nil {
		return User{}, err
	}
	name := strings.TrimSpace(in.FullName)
	if name == "" || len(name) > 200 {
		return User{}, ErrInvalidName
	}
	if len(in.Password) < 8 || len([]byte(in.Password)) > 72 {
		return User{}, ErrInvalidPassword
	}

	// Do the slow bcrypt work before the repository opens its transaction
	hash, err := s.pass.Hash(in.Password)
	if err != nil {
		return User{}, err
	}
	return s.repo.Create(ctx, email, hash, name)
}

func (s *Service) Authenticate(ctx context.Context, rawEmail, password string) (User, error) {
	email, err := cleanEmail(rawEmail)
	if err != nil || password == "" {
		return User{}, ErrInvalidCredentials
	}

	usr, err := s.repo.ByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return User{}, ErrInvalidCredentials
	}
	if err != nil {
		return User{}, err
	}
	if !s.pass.Matches(usr.PasswordHash, password) {
		return User{}, ErrInvalidCredentials
	}
	return usr, nil
}

func (s *Service) ByID(ctx context.Context, id int64) (User, error) {
	return s.repo.ByID(ctx, id)
}

func cleanEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	if email == "" || len(email) > 254 {
		return "", ErrInvalidEmail
	}
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || !strings.Contains(email, "@") {
		return "", ErrInvalidEmail
	}
	return email, nil
}

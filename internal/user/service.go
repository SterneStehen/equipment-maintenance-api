package user

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/SterneStehen/equipment-maintenance-api/internal/audit"
	"golang.org/x/crypto/bcrypt"
)

type store interface {
	Create(ctx context.Context, email, passwordHash, fullName string) (User, error)
	ByEmail(ctx context.Context, email string) (User, error)
	ByID(ctx context.Context, id int64) (User, error)
	List(ctx context.Context) ([]User, error)
	UpdateRole(ctx context.Context, id int64, role Role) (User, error)
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
	repo  store
	pass  passOps
	audit audit.Recorder
}

type RegisterInput struct {
	Email    string
	Password string
	FullName string
}

type Actor struct {
	UserID int64
	Role   Role
}

func NewService(repo store) *Service {
	return &Service{repo: repo, pass: bcryptPass{cost: bcrypt.DefaultCost}}
}

func NewServiceWithAudit(repo store, rec audit.Recorder) *Service {
	s := NewService(repo)
	s.audit = rec
	return s
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

	// Hash first; holding a db transaction while bcrypt works is just wasting time
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

func (s *Service) List(ctx context.Context, actor Actor) ([]User, error) {
	if err := s.needAdmin(ctx, actor); err != nil {
		return nil, err
	}
	return s.repo.List(ctx)
}

func (s *Service) Lookup(ctx context.Context, actor Actor, id int64) (User, error) {
	if err := s.needAdmin(ctx, actor); err != nil {
		return User{}, err
	}
	if id < 1 {
		return User{}, ErrNotFound
	}
	return s.repo.ByID(ctx, id)
}

func (s *Service) AssignRole(ctx context.Context, actor Actor, id int64, role Role) (User, error) {
	if err := s.needAdmin(ctx, actor); err != nil {
		return User{}, err
	}
	if id < 1 {
		return User{}, ErrNotFound
	}
	if !role.Valid() {
		return User{}, ErrInvalidRole
	}
	usr, err := s.repo.UpdateRole(ctx, id, role)
	if err != nil {
		return User{}, err
	}
	if s.audit != nil {
		_ = s.audit.Record(ctx, audit.EventInput{
			ActorID: actor.UserID, Action: "user.role_changed", Target: "user", TargetID: id, Details: "role=" + string(role),
		})
	}
	return usr, nil
}

func (s *Service) needAdmin(ctx context.Context, a Actor) error {
	if a.UserID < 1 {
		return ErrPermissionDenied
	}
	usr, err := s.repo.ByID(ctx, a.UserID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return ErrPermissionDenied
		}
		return err
	}
	if usr.Role != RoleAdmin {
		return ErrPermissionDenied
	}
	return nil
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

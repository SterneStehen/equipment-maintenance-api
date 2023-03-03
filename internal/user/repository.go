package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

const firstUsrLock int64 = 7319472104

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, email, passHash, name string) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, fmt.Errorf("begin user registration: %w", err)
	}
	defer tx.Rollback(ctx)

	role := RoleViewer
	var hasUsers bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users)").Scan(&hasUsers); err != nil {
		return User{}, fmt.Errorf("check initial user: %w", err)
	}

	if !hasUsers {
		// Several requests can see an empty table together; queue them here and check again
		if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock($1)", firstUsrLock); err != nil {
			return User{}, fmt.Errorf("lock initial administrator registration: %w", err)
		}
		if err := tx.QueryRow(ctx, "SELECT EXISTS (SELECT 1 FROM users)").Scan(&hasUsers); err != nil {
			return User{}, fmt.Errorf("recheck initial user: %w", err)
		}
		if !hasUsers {
			role = RoleAdmin
		}
	}

	usr, err := readUser(tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, full_name, role)
		VALUES ($1, $2, $3, $4)
		RETURNING id, email, password_hash, full_name, role, created_at, updated_at
	`, email, passHash, name, role))
	if err != nil {
		if isDup(err) {
			return User{}, ErrEmailTaken
		}
		return User{}, fmt.Errorf("insert user: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, fmt.Errorf("commit user registration: %w", err)
	}
	return usr, nil
}

func (r *Repository) ByEmail(ctx context.Context, email string) (User, error) {
	usr, err := readUser(r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, full_name, role, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user by email: %w", err)
	}
	return usr, nil
}

func (r *Repository) ByID(ctx context.Context, id int64) (User, error) {
	usr, err := readUser(r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, full_name, role, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("find user by id: %w", err)
	}
	return usr, nil
}

type scanRow interface {
	Scan(dest ...interface{}) error
}

func readUser(row scanRow) (User, error) {
	var usr User
	err := row.Scan(&usr.ID, &usr.Email, &usr.PasswordHash, &usr.FullName, &usr.Role, &usr.CreatedAt, &usr.UpdatedAt)
	return usr, err
}

func isDup(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

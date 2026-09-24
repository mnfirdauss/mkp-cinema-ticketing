package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mnfirdauss/mkp-cinema-ticketing/internal/model"
)

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

const userColumns = `id, full_name, email, phone, password_hash, role::text, cinema_id, is_active, last_login_at`

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*model.User, error) {
	return r.findOne(ctx, `SELECT `+userColumns+` FROM users WHERE lower(email) = lower($1)`, email)
}

func (r *UserRepository) FindByID(ctx context.Context, id string) (*model.User, error) {
	return r.findOne(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id)
}

func (r *UserRepository) TouchLastLogin(ctx context.Context, id string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET last_login_at = now() WHERE id = $1`, id)
	return err
}

func (r *UserRepository) findOne(ctx context.Context, query string, arg any) (*model.User, error) {
	var u model.User
	err := r.db.QueryRow(ctx, query, arg).Scan(
		&u.ID, &u.FullName, &u.Email, &u.Phone, &u.PasswordHash, &u.Role, &u.CinemaID, &u.IsActive, &u.LastLoginAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// AuthState returns the fields the auth middleware re-checks on every request.
func (r *UserRepository) AuthState(ctx context.Context, id string) (string, *int64, bool, error) {
	var (
		role     string
		cinemaID *int64
		active   bool
	)
	err := r.db.QueryRow(ctx, `SELECT role::text, cinema_id, is_active FROM users WHERE id = $1`, id).
		Scan(&role, &cinemaID, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil, false, ErrNotFound
	}
	return role, cinemaID, active, err
}

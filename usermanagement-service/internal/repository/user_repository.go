package repository

import (
	"context"
	"errors"
	"fmt"

	"diploma/usermanagement-service/internal/model"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrPhoneAlreadyExists = errors.New("phone already exists")
)

type CreateUserParams struct {
	FullName     string
	Email        string
	Phone        *string
	PasswordHash string
	Role         model.Role
	Status       model.Status
}

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) CreateUser(ctx context.Context, params CreateUserParams) (model.User, error) {
	const query = `
		INSERT INTO public.users (full_name, email, phone, password_hash, role, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING user_id, full_name, email, phone, password_hash, role, status, created_at;
	`

	var user model.User
	var role string
	var status string

	err := r.pool.QueryRow(
		ctx,
		query,
		params.FullName,
		params.Email,
		params.Phone,
		params.PasswordHash,
		params.Role,
		params.Status,
	).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&role,
		&status,
		&user.CreatedAt,
	)
	if err != nil {
		return model.User{}, mapDBError(err)
	}

	user.Role = model.Role(role)
	user.Status = model.Status(status)

	return user, nil
}

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key":
			return ErrEmailAlreadyExists
		case "users_phone_key":
			return ErrPhoneAlreadyExists
		}
	}
	return fmt.Errorf("database query failed: %w", err)
}

func (r *UserRepository) ActivateUser(ctx context.Context, userID int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET status='active' WHERE user_id=$1`,
		userID,
	)
	return err
}
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	const query = `
		SELECT user_id, full_name, email, phone, password_hash, role, status, created_at
		FROM users WHERE email=$1
	`

	var user model.User
	var role, status string

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&role,
		&status,
		&user.CreatedAt,
	)
	if err != nil {
		return model.User{}, err
	}

	user.Role = model.Role(role)
	user.Status = model.Status(status)

	return user, nil
}
func (r *UserRepository) GetByID(ctx context.Context, id int) (model.User, error) {
	const query = `
		SELECT user_id, full_name, email, phone, password_hash, role, status, created_at
		FROM users WHERE user_id=$1
	`

	var user model.User
	var role, status string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&role,
		&status,
		&user.CreatedAt,
	)
	if err != nil {
		return model.User{}, err
	}

	user.Role = model.Role(role)
	user.Status = model.Status(status)

	return user, nil
}
func (r *UserRepository) DeleteUser(ctx context.Context, userID int) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM users WHERE user_id=$1`,
		userID,
	)
	return err
}
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int, hash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1 WHERE user_id=$2`,
		hash,
		userID,
	)
	return err
}

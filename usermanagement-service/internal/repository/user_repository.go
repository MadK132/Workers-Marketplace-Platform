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

type UserSummary struct {
	ID        int          `json:"user_id"`
	FullName  string       `json:"full_name"`
	Email     string       `json:"email"`
	Phone     *string      `json:"phone,omitempty"`
	Role      model.Role   `json:"role"`
	Status    model.Status `json:"status"`
	CreatedAt string       `json:"created_at"`
}

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) EnsureManagerRole(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `ALTER TYPE public.user_role ADD VALUE IF NOT EXISTS 'manager'`)
	return err
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

func (r *UserRepository) EnsureDefaultAdmin(ctx context.Context, email, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.users (full_name, email, phone, password_hash, role, status)
		VALUES ('System Admin', $1, NULL, $2, 'admin', 'active')
		ON CONFLICT (email) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			role = 'admin',
			status = 'active';
	`, email, passwordHash)
	return err
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

func (r *UserRepository) ListUsers(ctx context.Context) ([]UserSummary, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT user_id, full_name, email, phone, role, status, created_at::text
		FROM users
		ORDER BY user_id DESC
		LIMIT 300
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]UserSummary, 0)
	for rows.Next() {
		var user UserSummary
		var role string
		var status string
		if err := rows.Scan(
			&user.ID,
			&user.FullName,
			&user.Email,
			&user.Phone,
			&role,
			&status,
			&user.CreatedAt,
		); err != nil {
			return nil, err
		}
		user.Role = model.Role(role)
		user.Status = model.Status(status)
		users = append(users, user)
	}
	return users, rows.Err()
}
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int, hash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1 WHERE user_id=$2`,
		hash,
		userID,
	)
	return err
}
func (r *UserRepository) UpdateRole(ctx context.Context, userID int, role model.Role) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET role=$1 WHERE user_id=$2`,
		role,
		userID,
	)
	return err
}

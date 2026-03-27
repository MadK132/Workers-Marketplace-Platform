package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type PasswordResetRepository struct {
	db *pgxpool.Pool
}

func NewPasswordResetRepository(db *pgxpool.Pool) *PasswordResetRepository {
	return &PasswordResetRepository{db: db}
}
func (r *PasswordResetRepository) Create(ctx context.Context, userID int, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO password_resets (user_id, token, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, token, expiresAt,
	)
	return err
}

func (r *PasswordResetRepository) GetByToken(ctx context.Context, token string) (int, time.Time, error) {
	var userID int
	var expiresAt time.Time

	err := r.db.QueryRow(ctx,
		`SELECT user_id, expires_at FROM password_resets WHERE token=$1`,
		token,
	).Scan(&userID, &expiresAt)

	return userID, expiresAt, err
}

func (r *PasswordResetRepository) Delete(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM password_resets WHERE token=$1`,
		token,
	)
	return err
}

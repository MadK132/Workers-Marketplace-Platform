package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type EmailVerificationRepository struct {
	db *pgxpool.Pool
}

func NewEmailVerificationRepository(db *pgxpool.Pool) *EmailVerificationRepository {
	return &EmailVerificationRepository{db: db}
}

func (r *EmailVerificationRepository) Create(ctx context.Context, userID int, token string, expiresAt time.Time) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO email_verifications (user_id, token, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, token, expiresAt,
	)
	return err
}

func (r *EmailVerificationRepository) GetByToken(ctx context.Context, token string) (int, time.Time, error) {
	var userID int
	var expiresAt time.Time

	err := r.db.QueryRow(ctx,
		`SELECT user_id, expires_at FROM email_verifications WHERE token=$1`,
		token,
	).Scan(&userID, &expiresAt)

	return userID, expiresAt, err
}

func (r *EmailVerificationRepository) Delete(ctx context.Context, token string) error {
	_, err := r.db.Exec(ctx,
		`DELETE FROM email_verifications WHERE token=$1`,
		token,
	)
	return err
}

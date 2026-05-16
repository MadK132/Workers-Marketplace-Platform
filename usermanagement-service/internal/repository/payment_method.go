package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPaymentMethodNotFound = errors.New("payment method not found")

type PaymentMethod struct {
	UserID   int    `json:"user_id"`
	Provider string `json:"provider"`
	Last4    string `json:"last4"`
}

type PaymentMethodRepository struct {
	db *pgxpool.Pool
}

func NewPaymentMethodRepository(db *pgxpool.Pool) *PaymentMethodRepository {
	return &PaymentMethodRepository{db: db}
}

func (r *PaymentMethodRepository) EnsureTable(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS user_payment_methods (
			user_id integer PRIMARY KEY REFERENCES users(user_id) ON DELETE CASCADE,
			provider varchar(50) NOT NULL DEFAULT 'stripe',
			card_last4 varchar(4) NOT NULL,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
			updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
		)
	`)
	return err
}

func (r *PaymentMethodRepository) Upsert(ctx context.Context, userID int, provider string, last4 string) (PaymentMethod, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return PaymentMethod{}, err
	}

	var method PaymentMethod
	err := r.db.QueryRow(ctx, `
		INSERT INTO user_payment_methods (user_id, provider, card_last4)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET
			provider = EXCLUDED.provider,
			card_last4 = EXCLUDED.card_last4,
			updated_at = CURRENT_TIMESTAMP
		RETURNING user_id, provider, card_last4
	`, userID, provider, last4).Scan(&method.UserID, &method.Provider, &method.Last4)
	return method, err
}

func (r *PaymentMethodRepository) Get(ctx context.Context, userID int) (PaymentMethod, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return PaymentMethod{}, err
	}

	var method PaymentMethod
	err := r.db.QueryRow(ctx, `
		SELECT user_id, provider, card_last4
		FROM user_payment_methods
		WHERE user_id = $1
	`, userID).Scan(&method.UserID, &method.Provider, &method.Last4)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PaymentMethod{}, ErrPaymentMethodNotFound
		}
		return PaymentMethod{}, err
	}
	return method, nil
}

func (r *PaymentMethodRepository) Exists(ctx context.Context, userID int) (bool, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return false, err
	}

	var exists bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_payment_methods WHERE user_id = $1
		)
	`, userID).Scan(&exists)
	return exists, err
}

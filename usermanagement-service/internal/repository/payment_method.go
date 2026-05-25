package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPaymentMethodNotFound = errors.New("payment method not found")

type PaymentMethod struct {
	ID        int       `json:"payment_method_id"`
	UserID    int       `json:"user_id"`
	Provider  string    `json:"provider"`
	Last4     string    `json:"last4"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
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
			payment_method_id serial PRIMARY KEY,
			user_id integer NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			provider varchar(50) NOT NULL DEFAULT 'stripe',
			card_last4 varchar(4) NOT NULL,
			is_active boolean NOT NULL DEFAULT false,
			created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
			updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
			UNIQUE (user_id, provider, card_last4)
		);

		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.columns
				WHERE table_name = 'user_payment_methods'
				  AND column_name = 'payment_method_id'
			) THEN
				CREATE TABLE user_payment_methods_new (
					payment_method_id serial PRIMARY KEY,
					user_id integer NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
					provider varchar(50) NOT NULL DEFAULT 'stripe',
					card_last4 varchar(4) NOT NULL,
					is_active boolean NOT NULL DEFAULT true,
					created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
					updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
					UNIQUE (user_id, provider, card_last4)
				);
				INSERT INTO user_payment_methods_new (user_id, provider, card_last4, is_active, created_at, updated_at)
				SELECT user_id, provider, card_last4, true, created_at, updated_at
				FROM user_payment_methods;
				DROP TABLE user_payment_methods;
				ALTER TABLE user_payment_methods_new RENAME TO user_payment_methods;
			END IF;
		END $$;

		CREATE UNIQUE INDEX IF NOT EXISTS idx_user_payment_methods_active
			ON user_payment_methods(user_id)
			WHERE is_active;
	`)
	return err
}

func (r *PaymentMethodRepository) Upsert(ctx context.Context, userID int, provider string, last4 string) (PaymentMethod, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return PaymentMethod{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return PaymentMethod{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE user_payment_methods
		SET is_active = false, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1
	`, userID); err != nil {
		return PaymentMethod{}, err
	}

	var method PaymentMethod
	err = tx.QueryRow(ctx, `
		INSERT INTO user_payment_methods (user_id, provider, card_last4, is_active)
		VALUES ($1, $2, $3, true)
		ON CONFLICT (user_id, provider, card_last4) DO UPDATE SET
			is_active = true,
			updated_at = CURRENT_TIMESTAMP
		RETURNING payment_method_id, user_id, provider, card_last4, is_active, created_at
	`, userID, provider, last4).Scan(&method.ID, &method.UserID, &method.Provider, &method.Last4, &method.IsActive, &method.CreatedAt)
	if err != nil {
		return PaymentMethod{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return PaymentMethod{}, err
	}
	return method, nil
}

func (r *PaymentMethodRepository) Get(ctx context.Context, userID int) (PaymentMethod, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return PaymentMethod{}, err
	}

	var method PaymentMethod
	err := r.db.QueryRow(ctx, `
		SELECT payment_method_id, user_id, provider, card_last4, is_active, created_at
		FROM user_payment_methods
		WHERE user_id = $1
		ORDER BY is_active DESC, updated_at DESC, payment_method_id DESC
		LIMIT 1
	`, userID).Scan(&method.ID, &method.UserID, &method.Provider, &method.Last4, &method.IsActive, &method.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return PaymentMethod{}, ErrPaymentMethodNotFound
		}
		return PaymentMethod{}, err
	}
	return method, nil
}

func (r *PaymentMethodRepository) List(ctx context.Context, userID int) ([]PaymentMethod, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return nil, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT payment_method_id, user_id, provider, card_last4, is_active, created_at
		FROM user_payment_methods
		WHERE user_id = $1
		ORDER BY is_active DESC, updated_at DESC, payment_method_id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var methods []PaymentMethod
	for rows.Next() {
		var method PaymentMethod
		if err := rows.Scan(&method.ID, &method.UserID, &method.Provider, &method.Last4, &method.IsActive, &method.CreatedAt); err != nil {
			return nil, err
		}
		methods = append(methods, method)
	}
	return methods, rows.Err()
}

func (r *PaymentMethodRepository) SetActive(ctx context.Context, userID int, paymentMethodID int) (PaymentMethod, error) {
	if err := r.EnsureTable(ctx); err != nil {
		return PaymentMethod{}, err
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return PaymentMethod{}, err
	}
	defer tx.Rollback(ctx)

	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM user_payment_methods
			WHERE user_id = $1 AND payment_method_id = $2
		)
	`, userID, paymentMethodID).Scan(&exists); err != nil {
		return PaymentMethod{}, err
	}
	if !exists {
		return PaymentMethod{}, ErrPaymentMethodNotFound
	}

	if _, err := tx.Exec(ctx, `
		UPDATE user_payment_methods
		SET is_active = false, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1
	`, userID); err != nil {
		return PaymentMethod{}, err
	}

	var method PaymentMethod
	if err := tx.QueryRow(ctx, `
		UPDATE user_payment_methods
		SET is_active = true, updated_at = CURRENT_TIMESTAMP
		WHERE user_id = $1 AND payment_method_id = $2
		RETURNING payment_method_id, user_id, provider, card_last4, is_active, created_at
	`, userID, paymentMethodID).Scan(&method.ID, &method.UserID, &method.Provider, &method.Last4, &method.IsActive, &method.CreatedAt); err != nil {
		return PaymentMethod{}, err
	}
	if err := tx.Commit(ctx); err != nil {
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

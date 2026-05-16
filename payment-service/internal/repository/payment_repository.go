package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrPaymentNotFound = errors.New("payment not found")

type Payment struct {
	ID                   int
	BookingID            int
	Amount               float64
	Currency             string
	Status               string
	Provider             string
	TransactionReference string
	PaidAt               *time.Time
}

type PaymentRepository struct {
	db *pgxpool.Pool
}

func NewPaymentRepository(db *pgxpool.Pool) *PaymentRepository {
	return &PaymentRepository{db: db}
}

func (r *PaymentRepository) EnsureTable(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		DO $$
		BEGIN
			IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payment_status') THEN
				CREATE TYPE payment_status AS ENUM ('pending', 'completed', 'failed', 'refunded');
			END IF;
		END $$;

		CREATE TABLE IF NOT EXISTS payments (
			payment_id serial PRIMARY KEY,
			booking_id integer UNIQUE REFERENCES bookings(booking_id) ON DELETE RESTRICT,
			amount numeric(10, 2),
			currency character(3) DEFAULT 'KZT',
			payment_status payment_status DEFAULT 'pending',
			payment_method varchar(50),
			transaction_reference varchar(255),
			paid_at timestamp without time zone
		)
	`)
	return err
}

func (r *PaymentRepository) Create(
	ctx context.Context,
	bookingID int,
	amount float64,
	currency string,
	provider string,
	transactionReference string,
) (Payment, error) {
	var payment Payment
	var paidAt *time.Time

	err := r.db.QueryRow(ctx, `
		INSERT INTO payments (
			booking_id,
			amount,
			currency,
			payment_status,
			payment_method,
			transaction_reference
		)
		VALUES ($1, $2, $3, 'pending', $4, $5)
		RETURNING
			payment_id,
			booking_id,
			amount::float8,
			TRIM(currency),
			payment_status::text,
			COALESCE(payment_method, ''),
			COALESCE(transaction_reference, ''),
			paid_at
	`, bookingID, amount, currency, provider, transactionReference).Scan(
		&payment.ID,
		&payment.BookingID,
		&payment.Amount,
		&payment.Currency,
		&payment.Status,
		&payment.Provider,
		&payment.TransactionReference,
		&paidAt,
	)
	if err != nil {
		return Payment{}, err
	}
	payment.PaidAt = paidAt

	return payment, nil
}

func (r *PaymentRepository) GetByBookingID(ctx context.Context, bookingID int) (Payment, error) {
	var payment Payment
	var paidAt *time.Time

	err := r.db.QueryRow(ctx, `
		SELECT
			payment_id,
			booking_id,
			amount::float8,
			TRIM(currency),
			payment_status::text,
			COALESCE(payment_method, ''),
			COALESCE(transaction_reference, ''),
			paid_at
		FROM payments
		WHERE booking_id = $1
	`, bookingID).Scan(
		&payment.ID,
		&payment.BookingID,
		&payment.Amount,
		&payment.Currency,
		&payment.Status,
		&payment.Provider,
		&payment.TransactionReference,
		&paidAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, err
	}
	payment.PaidAt = paidAt

	return payment, nil
}

func (r *PaymentRepository) GetByID(ctx context.Context, paymentID int) (Payment, error) {
	var payment Payment
	var paidAt *time.Time

	err := r.db.QueryRow(ctx, `
		SELECT
			payment_id,
			booking_id,
			amount::float8,
			TRIM(currency),
			payment_status::text,
			COALESCE(payment_method, ''),
			COALESCE(transaction_reference, ''),
			paid_at
		FROM payments
		WHERE payment_id = $1
	`, paymentID).Scan(
		&payment.ID,
		&payment.BookingID,
		&payment.Amount,
		&payment.Currency,
		&payment.Status,
		&payment.Provider,
		&payment.TransactionReference,
		&paidAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, err
	}
	payment.PaidAt = paidAt

	return payment, nil
}

func (r *PaymentRepository) UpdateStatus(
	ctx context.Context,
	paymentID int,
	status string,
	transactionReference string,
) (Payment, error) {
	var payment Payment
	var paidAt *time.Time

	err := r.db.QueryRow(ctx, `
		UPDATE payments
		SET
			payment_status = $2,
			transaction_reference = COALESCE(NULLIF($3, ''), transaction_reference),
			paid_at = CASE WHEN $2 = 'completed' THEN NOW() ELSE paid_at END
		WHERE payment_id = $1
		RETURNING
			payment_id,
			booking_id,
			amount::float8,
			TRIM(currency),
			payment_status::text,
			COALESCE(payment_method, ''),
			COALESCE(transaction_reference, ''),
			paid_at
	`, paymentID, status, transactionReference).Scan(
		&payment.ID,
		&payment.BookingID,
		&payment.Amount,
		&payment.Currency,
		&payment.Status,
		&payment.Provider,
		&payment.TransactionReference,
		&paidAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, err
	}
	payment.PaidAt = paidAt

	return payment, nil
}

func (r *PaymentRepository) UpdateStatusByTransactionReference(
	ctx context.Context,
	transactionReference string,
	status string,
) (Payment, error) {
	var payment Payment
	var paidAt *time.Time

	err := r.db.QueryRow(ctx, `
		UPDATE payments
		SET
			payment_status = $2,
			paid_at = CASE WHEN $2 = 'completed' THEN NOW() ELSE paid_at END
		WHERE transaction_reference = $1
		RETURNING
			payment_id,
			booking_id,
			amount::float8,
			TRIM(currency),
			payment_status::text,
			COALESCE(payment_method, ''),
			COALESCE(transaction_reference, ''),
			paid_at
	`, transactionReference, status).Scan(
		&payment.ID,
		&payment.BookingID,
		&payment.Amount,
		&payment.Currency,
		&payment.Status,
		&payment.Provider,
		&payment.TransactionReference,
		&paidAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Payment{}, ErrPaymentNotFound
		}
		return Payment{}, err
	}
	payment.PaidAt = paidAt

	return payment, nil
}

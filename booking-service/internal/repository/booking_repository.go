package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type BookingRepository struct {
	db *pgxpool.Pool
}

func NewBookingRepository(db *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{db: db}
}

func (r *BookingRepository) Create(
	ctx context.Context,
	requestID int,
	workerProfileID int,
) error {

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		INSERT INTO bookings (request_id, worker_profile_id)
		VALUES ($1, $2)
	`, requestID, workerProfileID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE service_requests
		SET status = 'assigned'
		WHERE request_id = $1
	`, requestID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}
func (r *BookingRepository) IsRequestAvailable(ctx context.Context, requestID int) (bool, error) {
	var status string

	err := r.db.QueryRow(ctx, `
		SELECT status FROM service_requests
		WHERE request_id = $1
	`, requestID).Scan(&status)

	if err != nil {
		return false, err
	}

	return status == "pending", nil
}

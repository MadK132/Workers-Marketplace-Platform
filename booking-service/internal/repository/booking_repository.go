package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrRequestNotFound = errors.New("request not found")
var ErrBookingNotFound = errors.New("booking not found")

type RequestForBooking struct {
	CustomerProfileID int
	CategoryID        int
	Status            string
}
type BookingDetails struct {
	RequestID       int
	WorkerProfileID int
	Status          string
}

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
		SET status = 'accepted'
		WHERE request_id = $1
	`, requestID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE worker_profiles
		SET is_available = false
		WHERE worker_profile_id = $1
	`, workerProfileID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *BookingRepository) GetRequestForBooking(
	ctx context.Context,
	requestID int,
) (RequestForBooking, error) {
	var req RequestForBooking

	err := r.db.QueryRow(ctx, `
		SELECT customer_profile_id, category_id, status
		FROM service_requests
		WHERE request_id = $1
	`, requestID).Scan(
		&req.CustomerProfileID,
		&req.CategoryID,
		&req.Status,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RequestForBooking{}, ErrRequestNotFound
		}
		return RequestForBooking{}, err
	}

	return req, nil
}

func (r *BookingRepository) IsWorkerEligible(
	ctx context.Context,
	workerProfileID int,
	categoryID int,
) (bool, error) {
	var exists bool

	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM worker_profiles wp
			JOIN worker_skills ws
			  ON ws.worker_profile_id = wp.worker_profile_id
			WHERE wp.worker_profile_id = $1
			  AND wp.verification_status = 'verified'
			  AND wp.is_available = true
			  AND ws.category_id = $2
			  AND ws.is_verified = true
		)
	`, workerProfileID, categoryID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

func (r *BookingRepository) GetBookingDetails(
	ctx context.Context,
	bookingID int,
) (BookingDetails, error) {
	var b BookingDetails
	err := r.db.QueryRow(ctx, `
		SELECT request_id, worker_profile_id, status
		FROM bookings
		WHERE booking_id = $1
	`, bookingID).Scan(&b.RequestID, &b.WorkerProfileID, &b.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BookingDetails{}, ErrBookingNotFound
		}
		return BookingDetails{}, err
	}
	return b, nil
}

func (r *BookingRepository) MarkInProgress(
	ctx context.Context,
	bookingID int,
	requestID int,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE bookings
		SET status = 'in_progress', start_time = NOW()
		WHERE booking_id = $1
	`, bookingID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE service_requests
		SET status = 'in_progress'
		WHERE request_id = $1
	`, requestID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func (r *BookingRepository) MarkCompleted(
	ctx context.Context,
	bookingID int,
	requestID int,
	workerProfileID int,
) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx, `
		UPDATE bookings
		SET status = 'completed', end_time = NOW()
		WHERE booking_id = $1
	`, bookingID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE service_requests
		SET status = 'completed'
		WHERE request_id = $1
	`, requestID)
	if err != nil {
		return err
	}

	_, err = tx.Exec(ctx, `
		UPDATE worker_profiles
		SET is_available = true
		WHERE worker_profile_id = $1
	`, workerProfileID)
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

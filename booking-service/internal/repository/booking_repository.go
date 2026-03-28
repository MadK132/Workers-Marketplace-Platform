package repository

import (
	"context"
	"errors"
	"time"

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

type BookingListItem struct {
	BookingID          int        `json:"booking_id"`
	RequestID          int        `json:"request_id"`
	WorkerProfileID    int        `json:"worker_profile_id"`
	CustomerProfileID  int        `json:"customer_profile_id"`
	CategoryID         int        `json:"category_id"`
	CategoryName       string     `json:"category_name"`
	RequestDescription string     `json:"request_description"`
	Status             string     `json:"status"`
	ScheduledTime      *time.Time `json:"scheduled_time,omitempty"`
	StartTime          *time.Time `json:"start_time,omitempty"`
	EndTime            *time.Time `json:"end_time,omitempty"`
	FinalPrice         *string    `json:"final_price,omitempty"`
	CounterpartyName   string     `json:"counterparty_name"`
	CounterpartyRole   string     `json:"counterparty_role"`
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

func (r *BookingRepository) ListByCustomerProfile(
	ctx context.Context,
	customerProfileID int,
) ([]BookingListItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			b.booking_id,
			b.request_id,
			b.worker_profile_id,
			sr.customer_profile_id,
			sr.category_id,
			COALESCE(sc.name, '') AS category_name,
			COALESCE(sr.description, '') AS request_description,
			b.status,
			b.scheduled_time,
			b.start_time,
			b.end_time,
			b.final_price::text AS final_price,
			COALESCE(wu.full_name, '') AS counterparty_name,
			'worker' AS counterparty_role
		FROM bookings b
		JOIN service_requests sr ON sr.request_id = b.request_id
		LEFT JOIN service_categories sc ON sc.category_id = sr.category_id
		LEFT JOIN worker_profiles wp ON wp.worker_profile_id = b.worker_profile_id
		LEFT JOIN users wu ON wu.user_id = wp.user_id
		WHERE sr.customer_profile_id = $1
		ORDER BY COALESCE(b.scheduled_time, sr.created_at) DESC, b.booking_id DESC
	`, customerProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]BookingListItem, 0)
	for rows.Next() {
		var item BookingListItem
		if err := rows.Scan(
			&item.BookingID,
			&item.RequestID,
			&item.WorkerProfileID,
			&item.CustomerProfileID,
			&item.CategoryID,
			&item.CategoryName,
			&item.RequestDescription,
			&item.Status,
			&item.ScheduledTime,
			&item.StartTime,
			&item.EndTime,
			&item.FinalPrice,
			&item.CounterpartyName,
			&item.CounterpartyRole,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *BookingRepository) ListByWorkerProfile(
	ctx context.Context,
	workerProfileID int,
) ([]BookingListItem, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			b.booking_id,
			b.request_id,
			b.worker_profile_id,
			sr.customer_profile_id,
			sr.category_id,
			COALESCE(sc.name, '') AS category_name,
			COALESCE(sr.description, '') AS request_description,
			b.status,
			b.scheduled_time,
			b.start_time,
			b.end_time,
			b.final_price::text AS final_price,
			COALESCE(cu.full_name, '') AS counterparty_name,
			'customer' AS counterparty_role
		FROM bookings b
		JOIN service_requests sr ON sr.request_id = b.request_id
		LEFT JOIN service_categories sc ON sc.category_id = sr.category_id
		LEFT JOIN customer_profiles cp ON cp.customer_profile_id = sr.customer_profile_id
		LEFT JOIN users cu ON cu.user_id = cp.user_id
		WHERE b.worker_profile_id = $1
		ORDER BY COALESCE(b.scheduled_time, sr.created_at) DESC, b.booking_id DESC
	`, workerProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]BookingListItem, 0)
	for rows.Next() {
		var item BookingListItem
		if err := rows.Scan(
			&item.BookingID,
			&item.RequestID,
			&item.WorkerProfileID,
			&item.CustomerProfileID,
			&item.CategoryID,
			&item.CategoryName,
			&item.RequestDescription,
			&item.Status,
			&item.ScheduledTime,
			&item.StartTime,
			&item.EndTime,
			&item.FinalPrice,
			&item.CounterpartyName,
			&item.CounterpartyRole,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

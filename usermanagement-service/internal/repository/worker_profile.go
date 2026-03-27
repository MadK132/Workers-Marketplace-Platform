package repository

import (
	"context"
	"diploma/usermanagement-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerProfileRepository struct {
	db *pgxpool.Pool
}

func NewWorkerProfileRepository(db *pgxpool.Pool) *WorkerProfileRepository {
	return &WorkerProfileRepository{db: db}
}

func (r *WorkerProfileRepository) Create(ctx context.Context, userID int) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO worker_profiles (user_id, verification_status)
		 VALUES ($1, 'pending')`,
		userID,
	)
	return err
}
func (r *WorkerProfileRepository) GetByUserID(ctx context.Context, userID int) (int, error) {
	var workerID int

	err := r.db.QueryRow(ctx,
		`SELECT worker_profile_id FROM worker_profiles WHERE user_id=$1`,
		userID,
	).Scan(&workerID)

	return workerID, err
}
func (r *WorkerProfileRepository) UpdateAvailability(ctx context.Context, workerID int, available bool) error {
	_, err := r.db.Exec(ctx,
		`UPDATE worker_profiles SET is_available=$1 WHERE worker_profile_id=$2`,
		available,
		workerID,
	)
	return err
}
func (r *WorkerProfileRepository) GetByUserIDFull(ctx context.Context, userID int) (model.WorkerProfile, error) {
	const query = `
		SELECT worker_profile_id, user_id, verification_status, is_available
		FROM worker_profiles
		WHERE user_id = $1
	`

	var worker model.WorkerProfile

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&worker.ID,
		&worker.UserID,
		&worker.VerificationStatus,
		&worker.IsAvailable,
	)

	return worker, err
}
func (r *WorkerProfileRepository) Verify(ctx context.Context, workerID int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE worker_profiles 
		 SET verification_status='verified'
		 WHERE worker_profile_id=$1`,
		workerID,
	)
	return err
}

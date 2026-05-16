package repository

import (
	"context"
	"diploma/usermanagement-service/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerProfileRepository struct {
	db *pgxpool.Pool
}

type VerifiedWorkerSkill struct {
	WorkerSkillID   int    `json:"worker_skill_id"`
	CategoryID      int    `json:"category_id"`
	CategoryName    string `json:"category_name"`
	ExperienceLevel string `json:"experience_level"`
	PriceBase       string `json:"price_base"`
}

func NewWorkerProfileRepository(db *pgxpool.Pool) *WorkerProfileRepository {
	return &WorkerProfileRepository{db: db}
}

func (r *WorkerProfileRepository) EnsureProfileColumns(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		ALTER TABLE worker_profiles
		ADD COLUMN IF NOT EXISTS profile_photo_url text
	`)
	return err
}

func (r *WorkerProfileRepository) Create(ctx context.Context, userID int) error {
	if err := r.EnsureProfileColumns(ctx); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx,
		`INSERT INTO worker_profiles (user_id, verification_status)
		 VALUES ($1, 'pending')
		 ON CONFLICT (user_id) DO NOTHING`,
		userID,
	)
	return err
}

func (r *WorkerProfileRepository) UpsertDetails(
	ctx context.Context,
	userID int,
	bio string,
	latitude *float64,
	longitude *float64,
	profilePhotoURL *string,
) (model.WorkerProfile, error) {
	if err := r.EnsureProfileColumns(ctx); err != nil {
		return model.WorkerProfile{}, err
	}

	query := `
		INSERT INTO worker_profiles
			(user_id, bio, current_latitude, current_longitude, profile_photo_url, verification_status)
		VALUES
			($1, $2, $3, $4, $5, 'pending')
		ON CONFLICT (user_id) DO UPDATE
		SET bio = EXCLUDED.bio,
		    current_latitude = COALESCE(EXCLUDED.current_latitude, worker_profiles.current_latitude),
		    current_longitude = COALESCE(EXCLUDED.current_longitude, worker_profiles.current_longitude),
		    profile_photo_url = COALESCE(EXCLUDED.profile_photo_url, worker_profiles.profile_photo_url)
		RETURNING
			worker_profile_id,
			user_id,
			COALESCE(bio, ''),
			COALESCE(rating, 0),
			verification_status,
			is_available,
			current_latitude,
			current_longitude,
			profile_photo_url
	`

	var worker model.WorkerProfile
	err := r.db.QueryRow(ctx, query, userID, bio, latitude, longitude, profilePhotoURL).Scan(
		&worker.ID,
		&worker.UserID,
		&worker.Bio,
		&worker.Rating,
		&worker.VerificationStatus,
		&worker.IsAvailable,
		&worker.CurrentLatitude,
		&worker.CurrentLongitude,
		&worker.ProfilePhotoURL,
	)
	return worker, err
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

func (r *WorkerProfileRepository) HasInProgressBooking(ctx context.Context, workerID int) (bool, error) {
	var exists bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1
			FROM bookings
			WHERE worker_profile_id = $1
			  AND status = 'in_progress'
		)`,
		workerID,
	).Scan(&exists)
	return exists, err
}

func (r *WorkerProfileRepository) GetByUserIDFull(ctx context.Context, userID int) (model.WorkerProfile, error) {
	if err := r.EnsureProfileColumns(ctx); err != nil {
		return model.WorkerProfile{}, err
	}
	const query = `
		SELECT
			worker_profile_id,
			user_id,
			COALESCE(bio, ''),
			COALESCE(rating, 0),
			verification_status,
			is_available,
			current_latitude,
			current_longitude,
			profile_photo_url
		FROM worker_profiles
		WHERE user_id = $1
	`

	var worker model.WorkerProfile

	err := r.db.QueryRow(ctx, query, userID).Scan(
		&worker.ID,
		&worker.UserID,
		&worker.Bio,
		&worker.Rating,
		&worker.VerificationStatus,
		&worker.IsAvailable,
		&worker.CurrentLatitude,
		&worker.CurrentLongitude,
		&worker.ProfilePhotoURL,
	)

	return worker, err
}

func (r *WorkerProfileRepository) ListVerifiedSkills(ctx context.Context, workerProfileID int) ([]VerifiedWorkerSkill, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			ws.worker_skill_id,
			ws.category_id,
			COALESCE(sc.name, '') AS category_name,
			ws.experience_level,
			ws.price_base::text
		FROM worker_skills ws
		JOIN service_categories sc ON sc.category_id = ws.category_id
		WHERE ws.worker_profile_id = $1
		  AND ws.is_verified = true
		ORDER BY sc.name, ws.worker_skill_id
	`, workerProfileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]VerifiedWorkerSkill, 0)
	for rows.Next() {
		var item VerifiedWorkerSkill
		if err := rows.Scan(
			&item.WorkerSkillID,
			&item.CategoryID,
			&item.CategoryName,
			&item.ExperienceLevel,
			&item.PriceBase,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
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

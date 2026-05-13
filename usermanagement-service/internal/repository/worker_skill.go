package repository

import (
	"context"
	"database/sql"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerSkillRepository struct {
	db *pgxpool.Pool
}
type WorkerSearchResult struct {
	WorkerID        int      `json:"worker_id"`
	FullName        string   `json:"full_name"`
	Price           int      `json:"price"`
	ExperienceLevel string   `json:"experience_level"`
	CategoryName    string   `json:"category_name"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
	DistanceMeters  *float64 `json:"distance_meters,omitempty"`
}

func NewWorkerSkillRepository(db *pgxpool.Pool) *WorkerSkillRepository {
	return &WorkerSkillRepository{db: db}
}

func (r *WorkerSkillRepository) Create(
	ctx context.Context,
	workerProfileID int,
	categoryID int,
	experience string,
	price int,
) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO worker_skills 
		(worker_profile_id, category_id, experience_level, price_base)
		VALUES ($1, $2, $3, $4)`,
		workerProfileID, categoryID, experience, price,
	)
	return err
}
func (r *WorkerSkillRepository) Verify(ctx context.Context, skillID int) error {
	_, err := r.db.Exec(ctx,
		`UPDATE worker_skills 
		 SET is_verified = true 
		 WHERE worker_skill_id = $1`,
		skillID,
	)
	return err
}
func (r *WorkerSkillRepository) FindWorkersByCategory(
	ctx context.Context,
	categoryID int,
) ([]WorkerSearchResult, error) {

	rows, err := r.db.Query(ctx, `
		SELECT 
			wp.worker_profile_id,
			u.full_name,
			ws.price_base,
			ws.experience_level,
			sc.name,
			wp.current_latitude::float8,
			wp.current_longitude::float8,
			NULL::float8 AS distance_meters
		FROM worker_profiles wp
		JOIN users u ON u.user_id = wp.user_id
		JOIN worker_skills ws ON ws.worker_profile_id = wp.worker_profile_id
		JOIN service_categories sc ON sc.category_id = ws.category_id
		WHERE ws.category_id = $1
		  AND ws.is_verified = true
		  AND wp.is_available = true
	`, categoryID)
	if err != nil {
		return nil, err
	}

	return scanWorkerSearchRows(rows)
}

func (r *WorkerSkillRepository) FindWorkersNearby(
	ctx context.Context,
	categoryID int,
	latitude float64,
	longitude float64,
	radiusMeters int,
) ([]WorkerSearchResult, error) {
	rows, err := r.db.Query(ctx, `
		WITH origin AS (
			SELECT ST_SetSRID(ST_MakePoint($3, $2), 4326)::geography AS point
		)
		SELECT
			wp.worker_profile_id,
			u.full_name,
			ws.price_base,
			ws.experience_level,
			sc.name,
			wp.current_latitude::float8,
			wp.current_longitude::float8,
			ST_Distance(wp.current_location, origin.point)::float8 AS distance_meters
		FROM worker_profiles wp
		JOIN users u ON u.user_id = wp.user_id
		JOIN worker_skills ws ON ws.worker_profile_id = wp.worker_profile_id
		JOIN service_categories sc ON sc.category_id = ws.category_id
		CROSS JOIN origin
		WHERE ws.category_id = $1
		  AND ws.is_verified = true
		  AND wp.is_available = true
		  AND wp.current_location IS NOT NULL
		  AND ST_DWithin(wp.current_location, origin.point, $4)
		ORDER BY distance_meters ASC, ws.price_base ASC
	`, categoryID, latitude, longitude, radiusMeters)
	if err != nil {
		return nil, err
	}

	return scanWorkerSearchRows(rows)
}

func scanWorkerSearchRows(rows pgx.Rows) ([]WorkerSearchResult, error) {
	defer rows.Close()

	result := make([]WorkerSearchResult, 0)
	for rows.Next() {
		var w WorkerSearchResult
		var latitude sql.NullFloat64
		var longitude sql.NullFloat64
		var distanceMeters sql.NullFloat64

		err := rows.Scan(
			&w.WorkerID,
			&w.FullName,
			&w.Price,
			&w.ExperienceLevel,
			&w.CategoryName,
			&latitude,
			&longitude,
			&distanceMeters,
		)
		if err != nil {
			return nil, err
		}
		if latitude.Valid {
			w.Latitude = &latitude.Float64
		}
		if longitude.Valid {
			w.Longitude = &longitude.Float64
		}
		if distanceMeters.Valid {
			w.DistanceMeters = &distanceMeters.Float64
		}

		result = append(result, w)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type WorkerSkillRepository struct {
	db *pgxpool.Pool
}
type WorkerSearchResult struct {
	WorkerID        int    `json:"worker_id"`
	FullName        string `json:"full_name"`
	Price           int    `json:"price"`
	ExperienceLevel string `json:"experience_level"`
	CategoryName    string `json:"category_name"`
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
			sc.name
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
	defer rows.Close()

	var result []WorkerSearchResult

	for rows.Next() {
		var w WorkerSearchResult

		err := rows.Scan(
			&w.WorkerID,
			&w.FullName,
			&w.Price,
			&w.ExperienceLevel,
			&w.CategoryName,
		)
		if err != nil {
			return nil, err
		}

		result = append(result, w)
	}

	return result, nil
}

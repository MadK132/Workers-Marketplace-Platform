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

func (r *WorkerSkillRepository) EnsureEvidenceTable(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS worker_skill_evidence (
			evidence_id serial PRIMARY KEY,
			worker_skill_id integer NOT NULL REFERENCES worker_skills(worker_skill_id) ON DELETE CASCADE,
			file_name varchar(255) NOT NULL,
			file_path text NOT NULL,
			content_type varchar(100),
			note text,
			created_at timestamp with time zone DEFAULT now()
		);

		CREATE INDEX IF NOT EXISTS idx_worker_skill_evidence_skill_id
			ON worker_skill_evidence(worker_skill_id);
	`)
	return err
}

func (r *WorkerSkillRepository) Create(
	ctx context.Context,
	workerProfileID int,
	categoryID int,
	experience string,
	price int,
) (int, error) {
	var skillID int
	err := r.db.QueryRow(ctx,
		`INSERT INTO worker_skills 
		(worker_profile_id, category_id, experience_level, price_base)
		VALUES ($1, $2, $3, $4)
		RETURNING worker_skill_id`,
		workerProfileID, categoryID, experience, price,
	).Scan(&skillID)
	return skillID, err
}

func (r *WorkerSkillRepository) AddEvidence(
	ctx context.Context,
	workerSkillID int,
	fileName string,
	filePath string,
	contentType string,
	note string,
) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO worker_skill_evidence
			(worker_skill_id, file_name, file_path, content_type, note)
		VALUES ($1, $2, $3, $4, $5)
	`, workerSkillID, fileName, filePath, contentType, note)
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

	result := make([]WorkerSearchResult, 0)
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

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

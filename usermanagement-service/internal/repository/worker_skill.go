package repository

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

		CREATE TABLE IF NOT EXISTS worker_skill_upgrade_requests (
			upgrade_request_id serial PRIMARY KEY,
			worker_skill_id integer NOT NULL REFERENCES worker_skills(worker_skill_id) ON DELETE CASCADE,
			worker_profile_id integer NOT NULL REFERENCES worker_profiles(worker_profile_id) ON DELETE CASCADE,
			requested_experience_level experience_level NOT NULL,
			evidence_files text NOT NULL DEFAULT '',
			admin_note text NOT NULL DEFAULT '',
			status varchar(20) NOT NULL DEFAULT 'pending'
				CHECK (status IN ('pending', 'approved', 'rejected')),
			created_at timestamp with time zone DEFAULT now(),
			reviewed_at timestamp with time zone,
			reviewed_by_user_id integer REFERENCES users(user_id) ON DELETE SET NULL
		);

		CREATE UNIQUE INDEX IF NOT EXISTS idx_worker_skill_upgrade_pending_unique
			ON worker_skill_upgrade_requests(worker_skill_id)
			WHERE status = 'pending';

		CREATE INDEX IF NOT EXISTS idx_worker_skill_upgrade_status
			ON worker_skill_upgrade_requests(status, created_at DESC);

		CREATE TABLE IF NOT EXISTS worker_identity_documents (
			identity_document_id serial PRIMARY KEY,
			worker_profile_id integer NOT NULL REFERENCES worker_profiles(worker_profile_id) ON DELETE CASCADE,
			file_name varchar(255) NOT NULL,
			file_path text NOT NULL,
			content_type varchar(100),
			status varchar(20) NOT NULL DEFAULT 'pending'
				CHECK (status IN ('pending', 'verified', 'rejected', 'replaced')),
			rejection_reason text,
			created_at timestamp with time zone DEFAULT now(),
			reviewed_at timestamp with time zone,
			reviewed_by_user_id integer REFERENCES users(user_id) ON DELETE SET NULL,
			assigned_manager_id integer REFERENCES users(user_id) ON DELETE SET NULL
		);

		ALTER TABLE worker_identity_documents
			ADD COLUMN IF NOT EXISTS assigned_manager_id integer REFERENCES users(user_id) ON DELETE SET NULL;

		ALTER TABLE worker_identity_documents
			DROP CONSTRAINT IF EXISTS worker_identity_documents_status_check;

		ALTER TABLE worker_identity_documents
			ADD CONSTRAINT worker_identity_documents_status_check
			CHECK (status IN ('pending', 'verified', 'rejected', 'replaced'));

		CREATE INDEX IF NOT EXISTS idx_worker_identity_documents_profile_status
			ON worker_identity_documents(worker_profile_id, status, created_at DESC);
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

func (r *WorkerSkillRepository) CreateUpgradeRequest(
	ctx context.Context,
	userID int,
	workerSkillID int,
	requestedLevel string,
	evidenceFiles string,
	note string,
) (int, error) {
	var currentLevel string
	var workerProfileID int
	err := r.db.QueryRow(ctx, `
		SELECT ws.experience_level, ws.worker_profile_id
		FROM worker_skills ws
		JOIN worker_profiles wp ON wp.worker_profile_id = ws.worker_profile_id
		WHERE ws.worker_skill_id = $1
		  AND wp.user_id = $2
		  AND ws.is_verified = true
	`, workerSkillID, userID).Scan(&currentLevel, &workerProfileID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, errors.New("verified worker skill not found")
		}
		return 0, err
	}

	if experienceRank(requestedLevel) <= experienceRank(currentLevel) {
		return 0, errors.New("requested level must be higher than current level")
	}

	var requestID int
	err = r.db.QueryRow(ctx, `
		INSERT INTO worker_skill_upgrade_requests
			(worker_skill_id, worker_profile_id, requested_experience_level, evidence_files, admin_note)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING upgrade_request_id
	`, workerSkillID, workerProfileID, requestedLevel, evidenceFiles, note).Scan(&requestID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return 0, errors.New("upgrade request is already pending")
	}
	return requestID, err
}

func (r *WorkerSkillRepository) ApproveUpgradeRequest(ctx context.Context, requestID int, reviewerUserID int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var workerSkillID int
	var workerProfileID int
	var requestedLevel string
	err = tx.QueryRow(ctx, `
		SELECT worker_skill_id, worker_profile_id, requested_experience_level
		FROM worker_skill_upgrade_requests
		WHERE upgrade_request_id = $1
		  AND status = 'pending'
		FOR UPDATE
	`, requestID).Scan(&workerSkillID, &workerProfileID, &requestedLevel)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("pending upgrade request not found")
		}
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE worker_skills
		SET experience_level = $2,
			is_verified = true
		WHERE worker_skill_id = $1
	`, workerSkillID, requestedLevel); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE worker_profiles
		SET verification_status = CASE
			WHEN EXISTS (
				SELECT 1 FROM worker_identity_documents
				WHERE worker_profile_id = $1 AND status = 'verified'
			)
			THEN 'verified'::verification_status
			ELSE 'pending'::verification_status
		END
		WHERE worker_profile_id = $1
	`, workerProfileID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE worker_skill_upgrade_requests
		SET status = 'approved',
			reviewed_at = now(),
			reviewed_by_user_id = $2
		WHERE upgrade_request_id = $1
	`, requestID, reviewerUserID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

func experienceRank(level string) int {
	switch level {
	case "junior":
		return 1
	case "middle":
		return 2
	case "senior":
		return 3
	default:
		return 0
	}
}

func (r *WorkerSkillRepository) Verify(ctx context.Context, skillID int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var workerProfileID int
	if err := tx.QueryRow(ctx, `
		UPDATE worker_skills
		SET is_verified = true
		WHERE worker_skill_id = $1
		RETURNING worker_profile_id
	`, skillID).Scan(&workerProfileID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE worker_profiles
		SET verification_status = CASE
			WHEN EXISTS (
				SELECT 1 FROM worker_identity_documents
				WHERE worker_profile_id = $1 AND status = 'verified'
			)
			THEN 'verified'::verification_status
			ELSE 'pending'::verification_status
		END
		WHERE worker_profile_id = $1
	`, workerProfileID); err != nil {
		return err
	}

	return tx.Commit(ctx)
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

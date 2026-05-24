package repository

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type AdminStats struct {
	UsersTotal             int `json:"users_total"`
	CustomersTotal         int `json:"customers_total"`
	WorkersTotal           int `json:"workers_total"`
	AdminsTotal            int `json:"admins_total"`
	PendingWorkerProfiles  int `json:"pending_worker_profiles"`
	VerifiedWorkerProfiles int `json:"verified_worker_profiles"`
	PendingWorkerSkills    int `json:"pending_worker_skills"`
	VerifiedWorkerSkills   int `json:"verified_worker_skills"`
	RequestsTotal          int `json:"requests_total"`
	BookingsTotal          int `json:"bookings_total"`
	BookingsInProgress     int `json:"bookings_in_progress"`
}

type PendingWorker struct {
	WorkerProfileID    int    `json:"worker_profile_id"`
	UserID             int    `json:"user_id"`
	FullName           string `json:"full_name"`
	Email              string `json:"email"`
	VerificationStatus string `json:"verification_status"`
}

type PendingIdentityDocument struct {
	IdentityDocumentID int    `json:"identity_document_id"`
	WorkerProfileID    int    `json:"worker_profile_id"`
	WorkerUserID       int    `json:"worker_user_id"`
	AssignedManagerID  *int   `json:"assigned_manager_id,omitempty"`
	WorkerFullName     string `json:"worker_full_name"`
	WorkerUserEmail    string `json:"worker_user_email"`
	FileName           string `json:"file_name"`
	FilePath           string `json:"file_path"`
	ContentType        string `json:"content_type"`
	CreatedAt          string `json:"created_at"`
}

type PendingSkill struct {
	WorkerSkillID   int    `json:"worker_skill_id"`
	WorkerProfileID int    `json:"worker_profile_id"`
	CategoryID      int    `json:"category_id"`
	CategoryName    string `json:"category_name"`
	ExperienceLevel string `json:"experience_level"`
	PriceBase       int    `json:"price_base"`
	IsVerified      bool   `json:"is_verified"`
	WorkerFullName  string `json:"worker_full_name"`
	WorkerUserID    int    `json:"worker_user_id"`
	WorkerUserEmail string `json:"worker_user_email"`
	EvidenceFiles   string `json:"evidence_files"`
}

type PendingSkillUpgrade struct {
	UpgradeRequestID int    `json:"upgrade_request_id"`
	WorkerSkillID    int    `json:"worker_skill_id"`
	WorkerProfileID  int    `json:"worker_profile_id"`
	CategoryName     string `json:"category_name"`
	CurrentLevel     string `json:"current_level"`
	RequestedLevel   string `json:"requested_level"`
	WorkerFullName   string `json:"worker_full_name"`
	WorkerUserEmail  string `json:"worker_user_email"`
	EvidenceFiles    string `json:"evidence_files"`
	AdminNote        string `json:"admin_note"`
	CreatedAt        string `json:"created_at"`
}

type AdminOverview struct {
	Stats                AdminStats                `json:"stats"`
	PendingWorkers       []PendingWorker           `json:"pending_workers"`
	PendingIdentities    []PendingIdentityDocument `json:"pending_identities"`
	PendingSkills        []PendingSkill            `json:"pending_skills"`
	PendingSkillUpgrades []PendingSkillUpgrade     `json:"pending_skill_upgrades"`
}

type AdminRepository struct {
	db *pgxpool.Pool
}

func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) GetOverview(ctx context.Context, userID int, role string) (AdminOverview, error) {
	var stats AdminStats
	err := r.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users) AS users_total,
			(SELECT COUNT(*) FROM users WHERE role = 'customer') AS customers_total,
			(SELECT COUNT(*) FROM users WHERE role = 'worker') AS workers_total,
			(SELECT COUNT(*) FROM users WHERE role = 'admin') AS admins_total,
			(SELECT COUNT(*) FROM worker_profiles WHERE verification_status = 'pending') AS pending_worker_profiles,
			(SELECT COUNT(*) FROM worker_profiles WHERE verification_status = 'verified') AS verified_worker_profiles,
			(SELECT COUNT(*) FROM worker_skills WHERE is_verified = false) AS pending_worker_skills,
			(SELECT COUNT(*) FROM worker_skills WHERE is_verified = true) AS verified_worker_skills,
			(SELECT COUNT(*) FROM service_requests) AS requests_total,
			(SELECT COUNT(*) FROM bookings) AS bookings_total,
			(SELECT COUNT(*) FROM bookings WHERE status = 'in_progress') AS bookings_in_progress
	`).Scan(
		&stats.UsersTotal,
		&stats.CustomersTotal,
		&stats.WorkersTotal,
		&stats.AdminsTotal,
		&stats.PendingWorkerProfiles,
		&stats.VerifiedWorkerProfiles,
		&stats.PendingWorkerSkills,
		&stats.VerifiedWorkerSkills,
		&stats.RequestsTotal,
		&stats.BookingsTotal,
		&stats.BookingsInProgress,
	)
	if err != nil {
		return AdminOverview{}, err
	}

	overview := AdminOverview{
		Stats:                stats,
		PendingWorkers:       make([]PendingWorker, 0),
		PendingIdentities:    make([]PendingIdentityDocument, 0),
		PendingSkills:        make([]PendingSkill, 0),
		PendingSkillUpgrades: make([]PendingSkillUpgrade, 0),
	}

	workerRows, err := r.db.Query(ctx, `
		SELECT
			wp.worker_profile_id,
			u.user_id,
			u.full_name,
			u.email,
			wp.verification_status
		FROM worker_profiles wp
		JOIN users u ON u.user_id = wp.user_id
		WHERE wp.verification_status = 'pending'
		ORDER BY wp.worker_profile_id DESC
		LIMIT 100
	`)
	if err != nil {
		return AdminOverview{}, err
	}
	defer workerRows.Close()

	for workerRows.Next() {
		var item PendingWorker
		if err := workerRows.Scan(
			&item.WorkerProfileID,
			&item.UserID,
			&item.FullName,
			&item.Email,
			&item.VerificationStatus,
		); err != nil {
			return AdminOverview{}, err
		}
		overview.PendingWorkers = append(overview.PendingWorkers, item)
	}
	if err := workerRows.Err(); err != nil {
		return AdminOverview{}, err
	}

	identityQuery := `
		SELECT
			wid.identity_document_id,
			wid.worker_profile_id,
			u.user_id,
			wid.assigned_manager_id,
			u.full_name,
			u.email,
			wid.file_name,
			wid.file_path,
			COALESCE(wid.content_type, ''),
			COALESCE(to_char(wid.created_at, 'YYYY-MM-DD HH24:MI'), '') AS created_at
		FROM worker_identity_documents wid
		JOIN worker_profiles wp ON wp.worker_profile_id = wid.worker_profile_id
		JOIN users u ON u.user_id = wp.user_id
		WHERE wid.status = 'pending'
	`
	identityArgs := []any{}
	if role == "manager" {
		identityQuery += ` AND (wid.assigned_manager_id IS NULL OR wid.assigned_manager_id = $1)`
		identityArgs = append(identityArgs, userID)
	}
	identityQuery += `
		ORDER BY wid.created_at DESC, wid.identity_document_id DESC
		LIMIT 200
	`
	identityRows, err := r.db.Query(ctx, identityQuery, identityArgs...)
	if err != nil {
		return AdminOverview{}, err
	}
	defer identityRows.Close()

	for identityRows.Next() {
		var item PendingIdentityDocument
		if err := identityRows.Scan(
			&item.IdentityDocumentID,
			&item.WorkerProfileID,
			&item.WorkerUserID,
			&item.AssignedManagerID,
			&item.WorkerFullName,
			&item.WorkerUserEmail,
			&item.FileName,
			&item.FilePath,
			&item.ContentType,
			&item.CreatedAt,
		); err != nil {
			return AdminOverview{}, err
		}
		overview.PendingIdentities = append(overview.PendingIdentities, item)
	}
	if err := identityRows.Err(); err != nil {
		return AdminOverview{}, err
	}

	skillRows, err := r.db.Query(ctx, `
		SELECT
			ws.worker_skill_id,
			ws.worker_profile_id,
			ws.category_id,
			COALESCE(sc.name, '') AS category_name,
			ws.experience_level,
			ws.price_base,
			ws.is_verified,
			u.full_name,
			u.user_id,
			u.email,
			COALESCE(string_agg(wse.file_path, ',' ORDER BY wse.evidence_id), '') AS evidence_files
		FROM worker_skills ws
		JOIN worker_profiles wp ON wp.worker_profile_id = ws.worker_profile_id
		JOIN users u ON u.user_id = wp.user_id
		LEFT JOIN service_categories sc ON sc.category_id = ws.category_id
		LEFT JOIN worker_skill_evidence wse ON wse.worker_skill_id = ws.worker_skill_id
		WHERE ws.is_verified = false
		GROUP BY ws.worker_skill_id, ws.worker_profile_id, ws.category_id, sc.name,
			ws.experience_level, ws.price_base, ws.is_verified, u.full_name, u.user_id, u.email
		ORDER BY ws.worker_skill_id DESC
		LIMIT 200
	`)
	if err != nil {
		return AdminOverview{}, err
	}
	defer skillRows.Close()

	for skillRows.Next() {
		var item PendingSkill
		if err := skillRows.Scan(
			&item.WorkerSkillID,
			&item.WorkerProfileID,
			&item.CategoryID,
			&item.CategoryName,
			&item.ExperienceLevel,
			&item.PriceBase,
			&item.IsVerified,
			&item.WorkerFullName,
			&item.WorkerUserID,
			&item.WorkerUserEmail,
			&item.EvidenceFiles,
		); err != nil {
			return AdminOverview{}, err
		}
		overview.PendingSkills = append(overview.PendingSkills, item)
	}
	if err := skillRows.Err(); err != nil {
		return AdminOverview{}, err
	}

	upgradeRows, err := r.db.Query(ctx, `
		SELECT
			wur.upgrade_request_id,
			wur.worker_skill_id,
			wur.worker_profile_id,
			COALESCE(sc.name, '') AS category_name,
			ws.experience_level AS current_level,
			wur.requested_experience_level,
			u.full_name,
			u.email,
			wur.evidence_files,
			wur.admin_note,
			COALESCE(to_char(wur.created_at, 'YYYY-MM-DD HH24:MI'), '') AS created_at
		FROM worker_skill_upgrade_requests wur
		JOIN worker_skills ws ON ws.worker_skill_id = wur.worker_skill_id
		JOIN worker_profiles wp ON wp.worker_profile_id = wur.worker_profile_id
		JOIN users u ON u.user_id = wp.user_id
		LEFT JOIN service_categories sc ON sc.category_id = ws.category_id
		WHERE wur.status = 'pending'
		ORDER BY wur.created_at DESC, wur.upgrade_request_id DESC
		LIMIT 200
	`)
	if err != nil {
		return AdminOverview{}, err
	}
	defer upgradeRows.Close()

	for upgradeRows.Next() {
		var item PendingSkillUpgrade
		if err := upgradeRows.Scan(
			&item.UpgradeRequestID,
			&item.WorkerSkillID,
			&item.WorkerProfileID,
			&item.CategoryName,
			&item.CurrentLevel,
			&item.RequestedLevel,
			&item.WorkerFullName,
			&item.WorkerUserEmail,
			&item.EvidenceFiles,
			&item.AdminNote,
			&item.CreatedAt,
		); err != nil {
			return AdminOverview{}, err
		}
		overview.PendingSkillUpgrades = append(overview.PendingSkillUpgrades, item)
	}
	if err := upgradeRows.Err(); err != nil {
		return AdminOverview{}, err
	}

	return overview, nil
}

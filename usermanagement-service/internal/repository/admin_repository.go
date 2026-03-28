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
}

type AdminOverview struct {
	Stats          AdminStats      `json:"stats"`
	PendingWorkers []PendingWorker `json:"pending_workers"`
	PendingSkills  []PendingSkill  `json:"pending_skills"`
}

type AdminRepository struct {
	db *pgxpool.Pool
}

func NewAdminRepository(db *pgxpool.Pool) *AdminRepository {
	return &AdminRepository{db: db}
}

func (r *AdminRepository) GetOverview(ctx context.Context) (AdminOverview, error) {
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
		Stats:          stats,
		PendingWorkers: make([]PendingWorker, 0),
		PendingSkills:  make([]PendingSkill, 0),
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
			u.email
		FROM worker_skills ws
		JOIN worker_profiles wp ON wp.worker_profile_id = ws.worker_profile_id
		JOIN users u ON u.user_id = wp.user_id
		LEFT JOIN service_categories sc ON sc.category_id = ws.category_id
		WHERE ws.is_verified = false
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
		); err != nil {
			return AdminOverview{}, err
		}
		overview.PendingSkills = append(overview.PendingSkills, item)
	}
	if err := skillRows.Err(); err != nil {
		return AdminOverview{}, err
	}

	return overview, nil
}

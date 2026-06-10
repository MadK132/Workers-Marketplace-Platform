package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"diploma/usermanagement-service/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrPhoneAlreadyExists = errors.New("phone already exists")
	ErrDeleteAdmin        = errors.New("admin accounts cannot be deleted")
)

type CreateUserParams struct {
	FullName     string
	Email        string
	Phone        *string
	PasswordHash string
	Role         model.Role
	Status       model.Status
}

type UserSummary struct {
	ID        int          `json:"user_id"`
	FullName  string       `json:"full_name"`
	Email     string       `json:"email"`
	Phone     *string      `json:"phone,omitempty"`
	Role      model.Role   `json:"role"`
	Status    model.Status `json:"status"`
	CreatedAt string       `json:"created_at"`
}

type StaffCustomerProfile struct {
	ID              int     `json:"customer_profile_id"`
	Address         *string `json:"address,omitempty"`
	Bio             string  `json:"bio"`
	ProfilePhotoURL *string `json:"profile_photo_url,omitempty"`
}

type StaffWorkerProfile struct {
	ID                 int     `json:"worker_profile_id"`
	Bio                string  `json:"bio"`
	Rating             float64 `json:"rating"`
	VerificationStatus string  `json:"verification_status"`
	IsAvailable        bool    `json:"is_available"`
	ProfilePhotoURL    *string `json:"profile_photo_url,omitempty"`
}

type StaffUserProfile struct {
	User              UserSummary              `json:"user"`
	CustomerProfile   *StaffCustomerProfile    `json:"customer_profile,omitempty"`
	WorkerProfile     *StaffWorkerProfile      `json:"worker_profile,omitempty"`
	VerifiedSkills    []VerifiedWorkerSkill    `json:"verified_skills,omitempty"`
	IdentityDocuments []WorkerIdentityDocument `json:"identity_documents,omitempty"`
	Penalties         []Penalty                `json:"penalties"`
	WarningCount      int                      `json:"warning_count"`
}

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) EnsureManagerRole(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `ALTER TYPE public.user_role ADD VALUE IF NOT EXISTS 'manager'`)
	return err
}

func (r *UserRepository) CreateUser(ctx context.Context, params CreateUserParams) (model.User, error) {
	const query = `
		INSERT INTO public.users (full_name, email, phone, password_hash, role, status)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING user_id, full_name, email, phone, password_hash, role, status, created_at;
	`

	var user model.User
	var role string
	var status string

	err := r.pool.QueryRow(
		ctx,
		query,
		params.FullName,
		params.Email,
		params.Phone,
		params.PasswordHash,
		params.Role,
		params.Status,
	).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&role,
		&status,
		&user.CreatedAt,
	)
	if err != nil {
		return model.User{}, mapDBError(err)
	}

	user.Role = model.Role(role)
	user.Status = model.Status(status)

	return user, nil
}

func (r *UserRepository) EnsureDefaultAdmin(ctx context.Context, email, passwordHash string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO public.users (full_name, email, phone, password_hash, role, status)
		VALUES ('System Admin', $1, NULL, $2, 'admin', 'active')
		ON CONFLICT (email) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			role = 'admin',
			status = 'active';
	`, email, passwordHash)
	return err
}

func mapDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "users_email_key":
			return ErrEmailAlreadyExists
		case "users_phone_key":
			return ErrPhoneAlreadyExists
		}
	}
	return fmt.Errorf("database query failed: %w", err)
}

func (r *UserRepository) ActivateUser(ctx context.Context, userID int) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET status='active' WHERE user_id=$1`,
		userID,
	)
	return err
}
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (model.User, error) {
	const query = `
		SELECT user_id, full_name, email, phone, password_hash, role, status, created_at
		FROM users WHERE email=$1
	`

	var user model.User
	var role, status string

	err := r.pool.QueryRow(ctx, query, email).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&role,
		&status,
		&user.CreatedAt,
	)
	if err != nil {
		return model.User{}, err
	}

	user.Role = model.Role(role)
	user.Status = model.Status(status)

	return user, nil
}
func (r *UserRepository) GetByID(ctx context.Context, id int) (model.User, error) {
	const query = `
		SELECT user_id, full_name, email, phone, password_hash, role, status, created_at
		FROM users WHERE user_id=$1
	`

	var user model.User
	var role, status string

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&user.ID,
		&user.FullName,
		&user.Email,
		&user.Phone,
		&user.PasswordHash,
		&role,
		&status,
		&user.CreatedAt,
	)
	if err != nil {
		return model.User{}, err
	}

	user.Role = model.Role(role)
	user.Status = model.Status(status)

	return user, nil
}
func (r *UserRepository) DeleteUser(ctx context.Context, userID int) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var role string
	if err := tx.QueryRow(ctx, `SELECT role::text FROM users WHERE user_id = $1 FOR UPDATE`, userID).Scan(&role); err != nil {
		return err
	}
	if role == string(model.RoleAdmin) {
		return ErrDeleteAdmin
	}

	if err := r.prepareDeletePreserveHistorySchema(ctx, tx); err != nil {
		return err
	}
	if err := r.prepareSoftDeleteSchema(ctx, tx); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE reports r
		SET reporter_name_snapshot = CASE
				WHEN r.reporter_user_id = $1 THEN COALESCE(NULLIF(r.reporter_name_snapshot, ''), u.full_name)
				ELSE r.reporter_name_snapshot
			END,
			reporter_email_snapshot = CASE
				WHEN r.reporter_user_id = $1 THEN COALESCE(NULLIF(r.reporter_email_snapshot, ''), u.email)
				ELSE r.reporter_email_snapshot
			END,
			reported_name_snapshot = CASE
				WHEN r.reported_user_id = $1 THEN COALESCE(NULLIF(r.reported_name_snapshot, ''), u.full_name)
				ELSE r.reported_name_snapshot
			END,
			reported_email_snapshot = CASE
				WHEN r.reported_user_id = $1 THEN COALESCE(NULLIF(r.reported_email_snapshot, ''), u.email)
				ELSE r.reported_email_snapshot
			END
		FROM users u
		WHERE u.user_id = $1
		  AND (r.reporter_user_id = $1 OR r.reported_user_id = $1)
	`, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		DELETE FROM chat_messages
		WHERE chat_id IN (
			SELECT chat_id FROM chats WHERE customer_user_id = $1 OR worker_user_id = $1
		)
	`, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM chats WHERE customer_user_id = $1 OR worker_user_id = $1`, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM notifications WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM email_verifications WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM password_resets WHERE user_id = $1`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM user_payment_methods WHERE user_id = $1`, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		UPDATE customer_profiles
		SET bio = '',
			profile_photo_url = NULL,
			latitude = NULL,
			longitude = NULL,
			location = NULL
		WHERE user_id = $1
	`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		WITH target_worker AS (
			SELECT worker_profile_id FROM worker_profiles WHERE user_id = $1
		)
		DELETE FROM worker_identity_documents
		WHERE worker_profile_id IN (SELECT worker_profile_id FROM target_worker)
	`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		WITH target_worker AS (
			SELECT worker_profile_id FROM worker_profiles WHERE user_id = $1
		)
		DELETE FROM worker_skill_upgrade_requests
		WHERE worker_profile_id IN (SELECT worker_profile_id FROM target_worker)
	`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		WITH target_worker AS (
			SELECT worker_profile_id FROM worker_profiles WHERE user_id = $1
		)
		DELETE FROM worker_skills
		WHERE worker_profile_id IN (SELECT worker_profile_id FROM target_worker)
	`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE worker_profiles
		SET bio = '',
			current_latitude = NULL,
			current_longitude = NULL,
			current_location = NULL,
			profile_photo_url = NULL,
			verification_status = 'unverified',
			is_available = false
		WHERE user_id = $1
	`, userID); err != nil {
		return err
	}

	deletedEmail := fmt.Sprintf("deleted-user-%d@deleted.local", userID)
	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET full_name = 'Deleted user',
			email = $2,
			phone = NULL,
			password_hash = '',
			status = 'inactive',
			deleted_at = COALESCE(deleted_at, NOW())
		WHERE user_id = $1
	`, userID, deletedEmail); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *UserRepository) HardDeleteUser(ctx context.Context, userID int) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM users WHERE user_id = $1`, userID)
	return err
}

func (r *UserRepository) prepareSoftDeleteSchema(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`)
	return err
}

func (r *UserRepository) prepareDeletePreserveHistorySchema(ctx context.Context, tx pgx.Tx) error {
	statements := []string{
		`ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_reporter_user_id_fkey`,
		`ALTER TABLE reports DROP CONSTRAINT IF EXISTS reports_reported_user_id_fkey`,
		`ALTER TABLE reports ADD COLUMN IF NOT EXISTS reporter_name_snapshot TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reports ADD COLUMN IF NOT EXISTS reporter_email_snapshot TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reports ADD COLUMN IF NOT EXISTS reported_name_snapshot TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reports ADD COLUMN IF NOT EXISTS reported_email_snapshot TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE reports ALTER COLUMN reporter_user_id DROP NOT NULL`,
		`ALTER TABLE reports ALTER COLUMN reported_user_id DROP NOT NULL`,
		`ALTER TABLE reports ADD CONSTRAINT reports_reporter_user_id_fkey FOREIGN KEY (reporter_user_id) REFERENCES users(user_id) ON DELETE SET NULL`,
		`ALTER TABLE reports ADD CONSTRAINT reports_reported_user_id_fkey FOREIGN KEY (reported_user_id) REFERENCES users(user_id) ON DELETE SET NULL`,
		`ALTER TABLE report_messages DROP CONSTRAINT IF EXISTS report_messages_sender_user_id_fkey`,
		`ALTER TABLE report_messages ALTER COLUMN sender_user_id DROP NOT NULL`,
		`ALTER TABLE report_messages ADD CONSTRAINT report_messages_sender_user_id_fkey FOREIGN KEY (sender_user_id) REFERENCES users(user_id) ON DELETE SET NULL`,
		`ALTER TABLE report_files DROP CONSTRAINT IF EXISTS report_files_uploaded_by_user_id_fkey`,
		`ALTER TABLE report_files ALTER COLUMN uploaded_by_user_id DROP NOT NULL`,
		`ALTER TABLE report_files ADD CONSTRAINT report_files_uploaded_by_user_id_fkey FOREIGN KEY (uploaded_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL`,
		`ALTER TABLE penalties DROP CONSTRAINT IF EXISTS penalties_user_id_fkey`,
		`ALTER TABLE penalties DROP CONSTRAINT IF EXISTS penalties_issued_by_user_id_fkey`,
		`ALTER TABLE penalties ALTER COLUMN user_id DROP NOT NULL`,
		`ALTER TABLE penalties ALTER COLUMN issued_by_user_id DROP NOT NULL`,
		`ALTER TABLE penalties ADD CONSTRAINT penalties_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE SET NULL`,
		`ALTER TABLE penalties ADD CONSTRAINT penalties_issued_by_user_id_fkey FOREIGN KEY (issued_by_user_id) REFERENCES users(user_id) ON DELETE SET NULL`,
		`ALTER TABLE payments DROP CONSTRAINT IF EXISTS payments_booking_id_fkey`,
		`ALTER TABLE payments ALTER COLUMN booking_id DROP NOT NULL`,
		`ALTER TABLE payments ADD CONSTRAINT payments_booking_id_fkey FOREIGN KEY (booking_id) REFERENCES bookings(booking_id) ON DELETE SET NULL`,
	}
	for _, stmt := range statements {
		if _, err := tx.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *UserRepository) ListUsers(ctx context.Context) ([]UserSummary, error) {
	if _, err := r.pool.Exec(ctx, `ALTER TABLE users ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMPTZ`); err != nil {
		return nil, err
	}
	rows, err := r.pool.Query(ctx, `
		SELECT user_id, full_name, email, phone, role, status, created_at::text
		FROM users
		WHERE deleted_at IS NULL
		ORDER BY user_id DESC
		LIMIT 300
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := make([]UserSummary, 0)
	for rows.Next() {
		var user UserSummary
		var role string
		var status string
		if err := rows.Scan(
			&user.ID,
			&user.FullName,
			&user.Email,
			&user.Phone,
			&role,
			&status,
			&user.CreatedAt,
		); err != nil {
			return nil, err
		}
		user.Role = model.Role(role)
		user.Status = model.Status(status)
		users = append(users, user)
	}
	return users, rows.Err()
}

func (r *UserRepository) GetStaffProfile(ctx context.Context, userID int) (StaffUserProfile, error) {
	if err := r.ensureStaffProfileColumns(ctx); err != nil {
		return StaffUserProfile{}, err
	}

	var result StaffUserProfile
	var role string
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT user_id, full_name, email, phone, role, status, created_at::text
		FROM users
		WHERE user_id = $1
	`, userID).Scan(
		&result.User.ID,
		&result.User.FullName,
		&result.User.Email,
		&result.User.Phone,
		&role,
		&status,
		&result.User.CreatedAt,
	)
	if err != nil {
		return StaffUserProfile{}, err
	}
	result.User.Role = model.Role(role)
	result.User.Status = model.Status(status)

	switch result.User.Role {
	case model.RoleCustomer:
		var profile StaffCustomerProfile
		err = r.pool.QueryRow(ctx, `
			SELECT customer_profile_id, address, COALESCE(bio, ''), profile_photo_url
			FROM customer_profiles
			WHERE user_id = $1
		`, userID).Scan(&profile.ID, &profile.Address, &profile.Bio, &profile.ProfilePhotoURL)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return StaffUserProfile{}, err
		}
		if err == nil {
			result.CustomerProfile = &profile
		}
	case model.RoleWorker:
		var profile StaffWorkerProfile
		err = r.pool.QueryRow(ctx, `
			SELECT worker_profile_id, COALESCE(bio, ''), COALESCE(rating, 0),
				verification_status, COALESCE(is_available, false), profile_photo_url
			FROM worker_profiles
			WHERE user_id = $1
		`, userID).Scan(
			&profile.ID,
			&profile.Bio,
			&profile.Rating,
			&profile.VerificationStatus,
			&profile.IsAvailable,
			&profile.ProfilePhotoURL,
		)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return StaffUserProfile{}, err
		}
		if err == nil {
			result.WorkerProfile = &profile
			docRows, err := r.pool.Query(ctx, `
				SELECT wid.identity_document_id, wid.worker_profile_id, wid.file_name, wid.file_path,
					COALESCE(wid.content_type, ''), wid.status, wid.created_at::text,
					wid.reviewed_at::text, reviewer.email, wid.rejection_reason
				FROM worker_identity_documents wid
				LEFT JOIN users reviewer ON reviewer.user_id = wid.reviewed_by_user_id
				WHERE wid.worker_profile_id = $1
				ORDER BY wid.created_at DESC, wid.identity_document_id DESC
				LIMIT 20
			`, profile.ID)
			if err != nil {
				return StaffUserProfile{}, err
			}
			defer docRows.Close()
			for docRows.Next() {
				var doc WorkerIdentityDocument
				if err := docRows.Scan(
					&doc.ID,
					&doc.WorkerProfileID,
					&doc.FileName,
					&doc.FilePath,
					&doc.ContentType,
					&doc.Status,
					&doc.UploadedAt,
					&doc.ReviewedAt,
					&doc.ReviewerEmail,
					&doc.RejectionReason,
				); err != nil {
					return StaffUserProfile{}, err
				}
				result.IdentityDocuments = append(result.IdentityDocuments, doc)
			}
			if err := docRows.Err(); err != nil {
				return StaffUserProfile{}, err
			}
		}
	default:
		return StaffUserProfile{}, errors.New("staff profiles are available only for customers and workers")
	}

	penalties, warningCount, err := r.listUserPenalties(ctx, userID)
	if err != nil {
		return StaffUserProfile{}, err
	}
	result.Penalties = penalties
	result.WarningCount = warningCount
	return result, nil
}

func (r *UserRepository) ensureStaffProfileColumns(ctx context.Context) error {
	_, err := r.pool.Exec(ctx, `
		ALTER TABLE customer_profiles
			ADD COLUMN IF NOT EXISTS bio text,
			ADD COLUMN IF NOT EXISTS profile_photo_url text;
		ALTER TABLE worker_profiles
			ADD COLUMN IF NOT EXISTS profile_photo_url text;
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

func (r *UserRepository) listUserPenalties(ctx context.Context, userID int) ([]Penalty, int, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT penalty_id, user_id, report_id, issued_by_user_id, penalty_type,
			reason, status, expires_at, created_at
		FROM penalties
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 100
	`, userID)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	penalties := make([]Penalty, 0)
	warnings := 0
	now := time.Now()
	for rows.Next() {
		var item Penalty
		var issuedByUserID sql.NullInt64
		if err := rows.Scan(
			&item.PenaltyID,
			&item.UserID,
			&item.ReportID,
			&issuedByUserID,
			&item.PenaltyType,
			&item.Reason,
			&item.Status,
			&item.ExpiresAt,
			&item.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		item.IssuedByUserID = intFromNullable(issuedByUserID)
		if item.PenaltyType == "warning" && item.Status == "active" && (item.ExpiresAt == nil || item.ExpiresAt.After(now)) {
			warnings++
		}
		penalties = append(penalties, item)
	}
	return penalties, warnings, rows.Err()
}
func (r *UserRepository) UpdatePassword(ctx context.Context, userID int, hash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash=$1 WHERE user_id=$2`,
		hash,
		userID,
	)
	return err
}
func (r *UserRepository) UpdateRole(ctx context.Context, userID int, role model.Role) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET role=$1 WHERE user_id=$2`,
		role,
		userID,
	)
	return err
}

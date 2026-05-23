package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrReportNotFound = errors.New("report not found")
	ErrReportAccess   = errors.New("report access denied")
)

type Report struct {
	ReportID          int       `json:"report_id"`
	BookingID         *int      `json:"booking_id,omitempty"`
	ReporterUserID    int       `json:"reporter_user_id"`
	ReportedUserID    int       `json:"reported_user_id"`
	AssignedManagerID *int      `json:"assigned_manager_id,omitempty"`
	Reason            string    `json:"reason"`
	Description       string    `json:"description"`
	Status            string    `json:"status"`
	Resolution        string    `json:"resolution,omitempty"`
	PenaltyType       string    `json:"penalty_type,omitempty"`
	ReporterName      string    `json:"reporter_name,omitempty"`
	ReportedName      string    `json:"reported_name,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type ReportFile struct {
	FileID           int       `json:"file_id"`
	ReportID         int       `json:"report_id"`
	FilePath         string    `json:"file_path"`
	FileName         string    `json:"file_name"`
	ContentType      string    `json:"content_type"`
	UploadedByUserID int       `json:"uploaded_by_user_id"`
	CreatedAt        time.Time `json:"created_at"`
}

type ReportMessage struct {
	MessageID      int        `json:"message_id"`
	ReportID       int        `json:"report_id"`
	SenderUserID   int        `json:"sender_user_id"`
	SenderName     string     `json:"sender_name,omitempty"`
	MessageText    string     `json:"message_text"`
	AttachmentURL  string     `json:"attachment_url,omitempty"`
	AttachmentName string     `json:"attachment_name,omitempty"`
	AttachmentType string     `json:"attachment_type,omitempty"`
	ReadAt         *time.Time `json:"read_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type Penalty struct {
	PenaltyID      int        `json:"penalty_id"`
	UserID         int        `json:"user_id"`
	ReportID       *int       `json:"report_id,omitempty"`
	IssuedByUserID int        `json:"issued_by_user_id"`
	PenaltyType    string     `json:"penalty_type"`
	Reason         string     `json:"reason"`
	Status         string     `json:"status"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type ReportRepository struct {
	db *pgxpool.Pool
}

func NewReportRepository(db *pgxpool.Pool) *ReportRepository {
	return &ReportRepository{db: db}
}

func (r *ReportRepository) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS reports (
			report_id SERIAL PRIMARY KEY,
			booking_id INTEGER REFERENCES bookings(booking_id) ON DELETE SET NULL,
			reporter_user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			reported_user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			assigned_manager_id INTEGER REFERENCES users(user_id) ON DELETE SET NULL,
			reason TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'open'
				CHECK (status IN ('open', 'in_review', 'waiting_customer', 'waiting_worker', 'resolved', 'rejected')),
			resolution TEXT NOT NULL DEFAULT '',
			penalty_type TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (reporter_user_id <> reported_user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS report_files (
			file_id SERIAL PRIMARY KEY,
			report_id INTEGER NOT NULL REFERENCES reports(report_id) ON DELETE CASCADE,
			file_path TEXT NOT NULL,
			file_name TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			uploaded_by_user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS report_messages (
			message_id SERIAL PRIMARY KEY,
			report_id INTEGER NOT NULL REFERENCES reports(report_id) ON DELETE CASCADE,
			sender_user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			message_text TEXT NOT NULL DEFAULT '',
			attachment_url TEXT NOT NULL DEFAULT '',
			attachment_name TEXT NOT NULL DEFAULT '',
			attachment_type TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (message_text <> '' OR attachment_url <> '')
		)`,
		`CREATE TABLE IF NOT EXISTS penalties (
			penalty_id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			report_id INTEGER REFERENCES reports(report_id) ON DELETE SET NULL,
			issued_by_user_id INTEGER NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
			penalty_type TEXT NOT NULL
				CHECK (penalty_type IN ('warning', 'temporary_suspend', 'block_user', 'unverify_skills')),
			reason TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL DEFAULT 'active'
				CHECK (status IN ('active', 'expired', 'cancelled')),
			expires_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			notification_id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(user_id) ON DELETE CASCADE,
			type VARCHAR(50),
			title VARCHAR(255),
			message TEXT,
			is_read BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`ALTER TABLE notifications
			ADD COLUMN IF NOT EXISTS action_type VARCHAR(50),
			ADD COLUMN IF NOT EXISTS action_ref VARCHAR(255),
			ADD COLUMN IF NOT EXISTS action_label VARCHAR(100),
			ADD COLUMN IF NOT EXISTS read_at TIMESTAMP`,
		`CREATE INDEX IF NOT EXISTS idx_reports_reporter ON reports(reporter_user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_reports_reported ON reports(reported_user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_reports_status ON reports(status, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_report_messages_report ON report_messages(report_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_penalties_user_active ON penalties(user_id, status, expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id_created_at ON notifications(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id_is_read ON notifications(user_id, is_read)`,
		`ALTER TABLE report_messages ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ`,
	}
	for _, stmt := range statements {
		if _, err := r.db.Exec(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *ReportRepository) BookingCounterparty(ctx context.Context, bookingID int, reporterUserID int) (int, error) {
	var customerUserID, workerUserID int
	err := r.db.QueryRow(ctx, `
		SELECT cp.user_id, wp.user_id
		FROM bookings b
		JOIN service_requests sr ON sr.request_id = b.request_id
		JOIN customer_profiles cp ON cp.customer_profile_id = sr.customer_profile_id
		JOIN worker_profiles wp ON wp.worker_profile_id = b.worker_profile_id
		WHERE b.booking_id = $1
	`, bookingID).Scan(&customerUserID, &workerUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrReportNotFound
	}
	if err != nil {
		return 0, err
	}
	switch reporterUserID {
	case customerUserID:
		return workerUserID, nil
	case workerUserID:
		return customerUserID, nil
	default:
		return 0, ErrReportAccess
	}
}

func (r *ReportRepository) Create(ctx context.Context, bookingID *int, reporterUserID int, reportedUserID int, reason string, description string) (Report, error) {
	var report Report
	err := r.db.QueryRow(ctx, `
		INSERT INTO reports (booking_id, reporter_user_id, reported_user_id, reason, description)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING report_id, booking_id, reporter_user_id, reported_user_id, assigned_manager_id,
			reason, description, status, resolution, penalty_type, created_at, updated_at
	`, bookingID, reporterUserID, reportedUserID, reason, description).Scan(
		&report.ReportID,
		&report.BookingID,
		&report.ReporterUserID,
		&report.ReportedUserID,
		&report.AssignedManagerID,
		&report.Reason,
		&report.Description,
		&report.Status,
		&report.Resolution,
		&report.PenaltyType,
		&report.CreatedAt,
		&report.UpdatedAt,
	)
	return report, err
}

func (r *ReportRepository) AddFile(ctx context.Context, reportID int, userID int, fileName string, filePath string, contentType string) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO report_files (report_id, uploaded_by_user_id, file_name, file_path, content_type)
		VALUES ($1, $2, $3, $4, $5)
	`, reportID, userID, fileName, filePath, contentType)
	return err
}

func (r *ReportRepository) NotifyStaffReportCreated(ctx context.Context, report Report) error {
	_, err := r.db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, action_type, action_ref, action_label)
		SELECT user_id, 'report_created', 'New report',
			'Report #' || $1::text || ' needs review.',
			'report', $1::text, 'Open report'
		FROM users
		WHERE role IN ('admin', 'manager')
		  AND status = 'active'
	`, report.ReportID)
	return err
}

func (r *ReportRepository) NotifyReportMessage(ctx context.Context, reportID int, senderUserID int, staff bool) error {
	if staff {
		_, err := r.db.Exec(ctx, `
			INSERT INTO notifications (user_id, type, title, message, action_type, action_ref, action_label)
			SELECT user_id, 'report_message', 'Support replied',
				'There is a new message in report #' || $1::text || '.',
				'report', $1::text, 'Open report'
			FROM reports
			JOIN users ON users.user_id IN (reports.reporter_user_id, reports.reported_user_id)
			WHERE reports.report_id = $1
			  AND users.user_id <> $2
		`, reportID, senderUserID)
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, action_type, action_ref, action_label)
		SELECT user_id, 'report_message', 'Report message',
			'New message in report #' || $1::text || '.',
			'report', $1::text, 'Open report'
		FROM users
		WHERE role IN ('admin', 'manager')
		  AND status = 'active'
		  AND user_id <> $2
	`, reportID, senderUserID)
	return err
}

func (r *ReportRepository) List(ctx context.Context, userID int, staff bool) ([]Report, error) {
	query := `
		SELECT r.report_id, r.booking_id, r.reporter_user_id, r.reported_user_id,
			r.assigned_manager_id, r.reason, r.description, r.status, r.resolution,
			r.penalty_type, reporter.full_name, reported.full_name, r.created_at, r.updated_at
		FROM reports r
		JOIN users reporter ON reporter.user_id = r.reporter_user_id
		JOIN users reported ON reported.user_id = r.reported_user_id
	`
	args := []any{}
	if !staff {
		query += ` WHERE r.reporter_user_id = $1 OR r.reported_user_id = $1`
		args = append(args, userID)
	}
	query += ` ORDER BY CASE WHEN r.status IN ('open', 'in_review') THEN 0 ELSE 1 END, r.updated_at DESC LIMIT 300`
	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports := make([]Report, 0)
	for rows.Next() {
		var report Report
		if err := rows.Scan(
			&report.ReportID,
			&report.BookingID,
			&report.ReporterUserID,
			&report.ReportedUserID,
			&report.AssignedManagerID,
			&report.Reason,
			&report.Description,
			&report.Status,
			&report.Resolution,
			&report.PenaltyType,
			&report.ReporterName,
			&report.ReportedName,
			&report.CreatedAt,
			&report.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (r *ReportRepository) CanAccess(ctx context.Context, reportID int, userID int, staff bool) error {
	if staff {
		return nil
	}
	var allowed bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM reports
			WHERE report_id = $1 AND (reporter_user_id = $2 OR reported_user_id = $2)
		)
	`, reportID, userID).Scan(&allowed)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrReportAccess
	}
	return nil
}

func (r *ReportRepository) ListMessages(ctx context.Context, reportID int, userID int, staff bool) ([]ReportMessage, error) {
	if err := r.CanAccess(ctx, reportID, userID, staff); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT rm.message_id, rm.report_id, rm.sender_user_id, u.full_name,
			rm.message_text, rm.attachment_url, rm.attachment_name, rm.attachment_type, rm.read_at, rm.created_at
		FROM report_messages rm
		JOIN users u ON u.user_id = rm.sender_user_id
		WHERE rm.report_id = $1
		ORDER BY rm.created_at, rm.message_id
	`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := make([]ReportMessage, 0)
	for rows.Next() {
		var msg ReportMessage
		if err := rows.Scan(
			&msg.MessageID,
			&msg.ReportID,
			&msg.SenderUserID,
			&msg.SenderName,
			&msg.MessageText,
			&msg.AttachmentURL,
			&msg.AttachmentName,
			&msg.AttachmentType,
			&msg.ReadAt,
			&msg.CreatedAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	return messages, rows.Err()
}

func (r *ReportRepository) AddMessage(ctx context.Context, reportID int, userID int, staff bool, text string, attachmentURL string, attachmentName string, attachmentType string) (ReportMessage, error) {
	if err := r.CanAccess(ctx, reportID, userID, staff); err != nil {
		return ReportMessage{}, err
	}
	var msg ReportMessage
	err := r.db.QueryRow(ctx, `
		WITH inserted AS (
			INSERT INTO report_messages (report_id, sender_user_id, message_text, attachment_url, attachment_name, attachment_type)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING message_id, report_id, sender_user_id, message_text, attachment_url, attachment_name, attachment_type, created_at
		), touched AS (
			UPDATE reports SET updated_at = NOW(), status = CASE WHEN status = 'open' THEN 'in_review' ELSE status END
			WHERE report_id = $1
		)
		SELECT i.message_id, i.report_id, i.sender_user_id, u.full_name,
			i.message_text, i.attachment_url, i.attachment_name, i.attachment_type, NULL::timestamptz, i.created_at
		FROM inserted i
		JOIN users u ON u.user_id = i.sender_user_id
	`, reportID, userID, strings.TrimSpace(text), attachmentURL, attachmentName, attachmentType).Scan(
		&msg.MessageID,
		&msg.ReportID,
		&msg.SenderUserID,
		&msg.SenderName,
		&msg.MessageText,
		&msg.AttachmentURL,
		&msg.AttachmentName,
		&msg.AttachmentType,
		&msg.ReadAt,
		&msg.CreatedAt,
	)
	return msg, err
}

func (r *ReportRepository) MarkMessagesRead(ctx context.Context, reportID int, userID int, staff bool) error {
	if err := r.CanAccess(ctx, reportID, userID, staff); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		UPDATE report_messages
		SET read_at = NOW()
		WHERE report_id = $1
		  AND sender_user_id <> $2
		  AND read_at IS NULL
	`, reportID, userID)
	return err
}

func (r *ReportRepository) ApplyPenalty(ctx context.Context, reportID int, targetUserID int, issuedByUserID int, penaltyType string, reason string, expiresAt *time.Time) (Penalty, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Penalty{}, err
	}
	defer tx.Rollback(ctx)

	var penalty Penalty
	err = tx.QueryRow(ctx, `
		INSERT INTO penalties (user_id, report_id, issued_by_user_id, penalty_type, reason, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING penalty_id, user_id, report_id, issued_by_user_id, penalty_type, reason, status, expires_at, created_at
	`, targetUserID, reportID, issuedByUserID, penaltyType, reason, expiresAt).Scan(
		&penalty.PenaltyID,
		&penalty.UserID,
		&penalty.ReportID,
		&penalty.IssuedByUserID,
		&penalty.PenaltyType,
		&penalty.Reason,
		&penalty.Status,
		&penalty.ExpiresAt,
		&penalty.CreatedAt,
	)
	if err != nil {
		return Penalty{}, err
	}

	switch penaltyType {
	case "block_user":
		if _, err = tx.Exec(ctx, `UPDATE users SET status = 'banned' WHERE user_id = $1`, targetUserID); err != nil {
			return Penalty{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE worker_profiles SET is_available = false WHERE user_id = $1`, targetUserID); err != nil {
			return Penalty{}, err
		}
	case "temporary_suspend":
		if _, err = tx.Exec(ctx, `UPDATE worker_profiles SET is_available = false WHERE user_id = $1`, targetUserID); err != nil {
			return Penalty{}, err
		}
	case "unverify_skills":
		if _, err = tx.Exec(ctx, `
			UPDATE worker_skills
			SET is_verified = false
			WHERE worker_profile_id IN (SELECT worker_profile_id FROM worker_profiles WHERE user_id = $1)
		`, targetUserID); err != nil {
			return Penalty{}, err
		}
		if _, err = tx.Exec(ctx, `UPDATE worker_profiles SET verification_status = 'unverified', is_available = false WHERE user_id = $1`, targetUserID); err != nil {
			return Penalty{}, err
		}
	}

	if _, err = tx.Exec(ctx, `
		UPDATE reports
		SET status = 'resolved', penalty_type = $1, resolution = $2, updated_at = NOW()
		WHERE report_id = $3
	`, penaltyType, reason, reportID); err != nil {
		return Penalty{}, err
	}

	if err = tx.Commit(ctx); err != nil {
		return Penalty{}, err
	}
	return penalty, nil
}

func (r *ReportRepository) CloseReport(ctx context.Context, reportID int, status string, resolution string) error {
	_, err := r.db.Exec(ctx, `
		UPDATE reports
		SET status = $1, resolution = $2, updated_at = NOW()
		WHERE report_id = $3
	`, status, resolution, reportID)
	return err
}

func (r *ReportRepository) HasActiveOnlineBlock(ctx context.Context, userID int) (bool, error) {
	var blocked bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM penalties
			WHERE user_id = $1
			  AND status = 'active'
			  AND penalty_type IN ('temporary_suspend', 'block_user', 'unverify_skills')
			  AND (expires_at IS NULL OR expires_at > NOW())
		)
	`, userID).Scan(&blocked)
	return blocked, err
}

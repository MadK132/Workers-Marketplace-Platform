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
	ErrReportClosed   = errors.New("report chat is closed")
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
	ReporterEmail     string    `json:"reporter_email,omitempty"`
	ReportedName      string    `json:"reported_name,omitempty"`
	ReportedEmail     string    `json:"reported_email,omitempty"`
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
	MessageID        int        `json:"message_id"`
	ReportID         int        `json:"report_id"`
	ConversationSide string     `json:"conversation_side"`
	SenderUserID     int        `json:"sender_user_id"`
	SenderName       string     `json:"sender_name,omitempty"`
	SenderEmail      string     `json:"sender_email,omitempty"`
	SenderRole       string     `json:"sender_role,omitempty"`
	SenderAvatarURL  string     `json:"sender_avatar_url,omitempty"`
	MessageText      string     `json:"message_text"`
	AttachmentURL    string     `json:"attachment_url,omitempty"`
	AttachmentName   string     `json:"attachment_name,omitempty"`
	AttachmentType   string     `json:"attachment_type,omitempty"`
	ReadAt           *time.Time `json:"read_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
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
			conversation_side TEXT NOT NULL DEFAULT 'reporter',
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
		`ALTER TABLE reports ADD COLUMN IF NOT EXISTS assigned_manager_id INTEGER REFERENCES users(user_id) ON DELETE SET NULL`,
		`CREATE INDEX IF NOT EXISTS idx_reports_assigned_manager ON reports(assigned_manager_id, status, updated_at DESC)`,
		`ALTER TABLE report_messages ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ`,
		`ALTER TABLE report_messages ADD COLUMN IF NOT EXISTS conversation_side TEXT NOT NULL DEFAULT 'reporter'`,
		`CREATE INDEX IF NOT EXISTS idx_report_messages_report ON report_messages(report_id, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_report_messages_report_side ON report_messages(report_id, conversation_side, created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_penalties_user_active ON penalties(user_id, status, expires_at)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id_created_at ON notifications(user_id, created_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_id_is_read ON notifications(user_id, is_read)`,
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
	if _, err := r.db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, action_type, action_ref, action_label)
		SELECT user_id, 'report_created', 'New report',
			'Report #' || $1::integer::text || ' needs review.',
			'report', $1::integer::text, 'Open report'
		FROM users
		WHERE role IN ('admin', 'manager')
		  AND status = 'active'
	`, report.ReportID); err != nil {
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, action_type, action_ref, action_label)
		VALUES ($1, 'report_created', 'Report opened',
			'A support case was opened for one of your bookings.',
			'report', $2::integer::text, 'Open report')
	`, report.ReportedUserID, report.ReportID)
	return err
}

func (r *ReportRepository) NotifyReportMessage(ctx context.Context, reportID int, senderUserID int, staff bool, conversationSide string) error {
	conversationSide = normalizeReportConversationSide(conversationSide)
	if staff {
		_, err := r.db.Exec(ctx, `
			INSERT INTO notifications (user_id, type, title, message, action_type, action_ref, action_label)
			SELECT user_id, 'report_message', 'Support replied',
				'There is a new message in report #' || $1::integer::text || '.',
				'report', $1::integer::text, 'Open report'
			FROM reports
			JOIN users ON users.user_id = CASE WHEN $3 = 'reported' THEN reports.reported_user_id ELSE reports.reporter_user_id END
			WHERE reports.report_id = $1
			  AND users.user_id <> $2
		`, reportID, senderUserID, conversationSide)
		return err
	}
	_, err := r.db.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, action_type, action_ref, action_label)
		SELECT u.user_id, 'report_message', 'Report message',
			'New message in report #' || $1::integer::text || '.',
			'report', $1::integer::text, 'Open report'
		FROM reports r
		JOIN users u ON (
			(r.assigned_manager_id IS NOT NULL AND u.user_id = r.assigned_manager_id)
			OR (r.assigned_manager_id IS NULL AND u.role IN ('admin', 'manager'))
		)
		WHERE r.report_id = $1
		  AND u.status = 'active'
		  AND u.user_id <> $2
	`, reportID, senderUserID)
	return err
}

func (r *ReportRepository) Assign(ctx context.Context, reportID int, managerUserID int) (Report, error) {
	var report Report
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return Report{}, err
	}
	defer tx.Rollback(ctx)

	err = tx.QueryRow(ctx, `
		UPDATE reports
		SET assigned_manager_id = $2,
			status = CASE WHEN status = 'open' THEN 'in_review' ELSE status END,
			updated_at = NOW()
		WHERE report_id = $1
		  AND status IN ('open', 'in_review', 'waiting_customer', 'waiting_worker')
		  AND (assigned_manager_id IS NULL OR assigned_manager_id = $2)
		RETURNING report_id, booking_id, reporter_user_id, reported_user_id, assigned_manager_id,
			reason, description, status, resolution, penalty_type, created_at, updated_at
	`, reportID, managerUserID).Scan(
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
	if err != nil {
		return Report{}, err
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO notifications (user_id, type, title, message, action_type, action_ref, action_label)
		VALUES ($1, 'report_assigned', 'Report assigned',
			'Report #' || $2::integer::text || ' is assigned to you.',
			'report', $2::integer::text, 'Open report')
	`, managerUserID, reportID); err != nil {
		return Report{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Report{}, err
	}
	return report, nil
}

func (r *ReportRepository) List(ctx context.Context, userID int, staff bool, role string) ([]Report, error) {
	query := `
		SELECT r.report_id, r.booking_id, r.reporter_user_id, r.reported_user_id,
			r.assigned_manager_id, r.reason, r.description, r.status, r.resolution,
			r.penalty_type, reporter.full_name, reporter.email, reported.full_name, reported.email,
			r.created_at, r.updated_at
		FROM reports r
		JOIN users reporter ON reporter.user_id = r.reporter_user_id
		JOIN users reported ON reported.user_id = r.reported_user_id
	`
	args := []any{}
	if staff && role == "manager" {
		query += ` WHERE r.assigned_manager_id IS NULL OR r.assigned_manager_id = $1`
		args = append(args, userID)
	} else if !staff {
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
			&report.ReporterEmail,
			&report.ReportedName,
			&report.ReportedEmail,
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

func normalizeReportConversationSide(side string) string {
	side = strings.ToLower(strings.TrimSpace(side))
	if side != "reported" {
		return "reporter"
	}
	return side
}

func (r *ReportRepository) CanAccessConversation(ctx context.Context, reportID int, userID int, staff bool, side string) (string, error) {
	side = normalizeReportConversationSide(side)
	if staff {
		return side, nil
	}
	var reporterUserID, reportedUserID int
	err := r.db.QueryRow(ctx, `
		SELECT reporter_user_id, reported_user_id
		FROM reports
		WHERE report_id = $1
	`, reportID).Scan(&reporterUserID, &reportedUserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrReportNotFound
	}
	if err != nil {
		return "", err
	}
	switch {
	case userID == reporterUserID:
		return "reporter", nil
	case userID == reportedUserID:
		return "reported", nil
	default:
		return "", ErrReportAccess
	}
}

func (r *ReportRepository) ListMessages(ctx context.Context, reportID int, userID int, staff bool, conversationSide string) ([]ReportMessage, error) {
	side, err := r.CanAccessConversation(ctx, reportID, userID, staff, conversationSide)
	if err != nil {
		return nil, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT rm.message_id, rm.report_id, rm.conversation_side, rm.sender_user_id,
			u.full_name, u.email, u.role::text,
			COALESCE(cp.profile_photo_url, wp.profile_photo_url, '') AS sender_avatar_url,
			rm.message_text, rm.attachment_url, rm.attachment_name, rm.attachment_type, rm.read_at, rm.created_at
		FROM report_messages rm
		JOIN users u ON u.user_id = rm.sender_user_id
		LEFT JOIN customer_profiles cp ON cp.user_id = rm.sender_user_id
		LEFT JOIN worker_profiles wp ON wp.user_id = rm.sender_user_id
		WHERE rm.report_id = $1
		  AND rm.conversation_side = $2
		ORDER BY rm.created_at, rm.message_id
	`, reportID, side)
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
			&msg.ConversationSide,
			&msg.SenderUserID,
			&msg.SenderName,
			&msg.SenderEmail,
			&msg.SenderRole,
			&msg.SenderAvatarURL,
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

func (r *ReportRepository) AddMessage(ctx context.Context, reportID int, userID int, staff bool, conversationSide string, text string, attachmentURL string, attachmentName string, attachmentType string) (ReportMessage, error) {
	side, err := r.CanAccessConversation(ctx, reportID, userID, staff, conversationSide)
	if err != nil {
		return ReportMessage{}, err
	}
	var msg ReportMessage
	err = r.db.QueryRow(ctx, `
		WITH allowed_report AS (
			SELECT report_id
			FROM reports
			WHERE report_id = $1
			  AND status NOT IN ('resolved', 'rejected')
		), inserted AS (
			INSERT INTO report_messages (report_id, conversation_side, sender_user_id, message_text, attachment_url, attachment_name, attachment_type)
			SELECT report_id, $2, $3, $4, $5, $6, $7
			FROM allowed_report
			RETURNING message_id, report_id, conversation_side, sender_user_id, message_text, attachment_url, attachment_name, attachment_type, created_at
		), touched AS (
			UPDATE reports SET updated_at = NOW(), status = CASE WHEN status = 'open' THEN 'in_review' ELSE status END
			WHERE report_id = $1
		)
		SELECT i.message_id, i.report_id, i.conversation_side, i.sender_user_id,
			u.full_name, u.email, u.role::text,
			COALESCE(cp.profile_photo_url, wp.profile_photo_url, '') AS sender_avatar_url,
			i.message_text, i.attachment_url, i.attachment_name, i.attachment_type, NULL::timestamptz, i.created_at
		FROM inserted i
		JOIN users u ON u.user_id = i.sender_user_id
		LEFT JOIN customer_profiles cp ON cp.user_id = i.sender_user_id
		LEFT JOIN worker_profiles wp ON wp.user_id = i.sender_user_id
	`, reportID, side, userID, strings.TrimSpace(text), attachmentURL, attachmentName, attachmentType).Scan(
		&msg.MessageID,
		&msg.ReportID,
		&msg.ConversationSide,
		&msg.SenderUserID,
		&msg.SenderName,
		&msg.SenderEmail,
		&msg.SenderRole,
		&msg.SenderAvatarURL,
		&msg.MessageText,
		&msg.AttachmentURL,
		&msg.AttachmentName,
		&msg.AttachmentType,
		&msg.ReadAt,
		&msg.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return ReportMessage{}, ErrReportClosed
	}
	return msg, err
}

func (r *ReportRepository) MarkMessagesRead(ctx context.Context, reportID int, userID int, staff bool, conversationSide string) error {
	side, err := r.CanAccessConversation(ctx, reportID, userID, staff, conversationSide)
	if err != nil {
		return err
	}
	_, err = r.db.Exec(ctx, `
		UPDATE report_messages
		SET read_at = NOW()
		WHERE report_id = $1
		  AND conversation_side = $2
		  AND sender_user_id <> $3
		  AND read_at IS NULL
	`, reportID, side, userID)
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
		var workerProfileID int
		err = tx.QueryRow(ctx, `
			WITH booking_skill AS (
				SELECT b.worker_profile_id, sr.category_id
				FROM reports r
				JOIN bookings b ON b.booking_id = r.booking_id
				JOIN service_requests sr ON sr.request_id = b.request_id
				JOIN worker_profiles wp ON wp.worker_profile_id = b.worker_profile_id
				WHERE r.report_id = $1
				  AND wp.user_id = $2
			),
			updated AS (
				UPDATE worker_skills ws
				SET is_verified = false
				FROM booking_skill bs
				WHERE ws.worker_profile_id = bs.worker_profile_id
				  AND ws.category_id = bs.category_id
				  AND ws.is_verified = true
				RETURNING ws.worker_profile_id
			)
			SELECT worker_profile_id FROM updated LIMIT 1
		`, reportID, targetUserID).Scan(&workerProfileID)
		if errors.Is(err, pgx.ErrNoRows) {
			return Penalty{}, errors.New("booking worker skill not found or already unverified")
		}
		if err != nil {
			return Penalty{}, err
		}
		if _, err = tx.Exec(ctx, `
			UPDATE worker_profiles wp
			SET verification_status = CASE
					WHEN EXISTS (
						SELECT 1 FROM worker_identity_documents wid
						WHERE wid.worker_profile_id = wp.worker_profile_id
						  AND wid.status = 'verified'
					)
					AND EXISTS (
						SELECT 1 FROM worker_skills ws
						WHERE ws.worker_profile_id = wp.worker_profile_id
						  AND ws.is_verified = true
					)
					THEN 'verified'::verification_status
					ELSE 'unverified'::verification_status
				END,
				is_available = CASE
					WHEN EXISTS (
						SELECT 1 FROM worker_identity_documents wid
						WHERE wid.worker_profile_id = wp.worker_profile_id
						  AND wid.status = 'verified'
					)
					AND EXISTS (
						SELECT 1 FROM worker_skills ws
						WHERE ws.worker_profile_id = wp.worker_profile_id
						  AND ws.is_verified = true
					)
					THEN wp.is_available
					ELSE false
				END
			WHERE wp.worker_profile_id = $1
		`, workerProfileID); err != nil {
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

func (r *ReportRepository) CancelPenalty(ctx context.Context, penaltyID int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var userID int
	var penaltyType string
	err = tx.QueryRow(ctx, `
		UPDATE penalties
		SET status = 'cancelled'
		WHERE penalty_id = $1 AND status = 'active'
		RETURNING user_id, penalty_type
	`, penaltyID).Scan(&userID, &penaltyType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrReportNotFound
		}
		return err
	}

	if penaltyType == "block_user" {
		var hasBlock bool
		if err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM penalties
				WHERE user_id = $1
				  AND status = 'active'
				  AND penalty_type = 'block_user'
				  AND (expires_at IS NULL OR expires_at > NOW())
			)
		`, userID).Scan(&hasBlock); err != nil {
			return err
		}
		if !hasBlock {
			if _, err = tx.Exec(ctx, `UPDATE users SET status = 'active' WHERE user_id = $1 AND status = 'banned'`, userID); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
}

func (r *ReportRepository) ExpireExpiredPenalties(ctx context.Context, userID int) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		UPDATE penalties
		SET status = 'expired'
		WHERE user_id = $1
		  AND status = 'active'
		  AND expires_at IS NOT NULL
		  AND expires_at <= NOW()
		RETURNING penalty_type
	`, userID)
	if err != nil {
		return err
	}
	defer rows.Close()

	expiredBlock := false
	for rows.Next() {
		var penaltyType string
		if err := rows.Scan(&penaltyType); err != nil {
			return err
		}
		if penaltyType == "block_user" {
			expiredBlock = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if expiredBlock {
		var hasBlock bool
		if err = tx.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM penalties
				WHERE user_id = $1
				  AND status = 'active'
				  AND penalty_type = 'block_user'
				  AND (expires_at IS NULL OR expires_at > NOW())
			)
		`, userID).Scan(&hasBlock); err != nil {
			return err
		}
		if !hasBlock {
			if _, err = tx.Exec(ctx, `UPDATE users SET status = 'active' WHERE user_id = $1 AND status = 'banned'`, userID); err != nil {
				return err
			}
		}
	}

	return tx.Commit(ctx)
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
	if err := r.ExpireExpiredPenalties(ctx, userID); err != nil {
		return false, err
	}
	var blocked bool
	err := r.db.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM penalties
			WHERE user_id = $1
			  AND status = 'active'
			  AND penalty_type IN ('temporary_suspend', 'block_user')
			  AND (expires_at IS NULL OR expires_at > NOW())
		)
	`, userID).Scan(&blocked)
	return blocked, err
}

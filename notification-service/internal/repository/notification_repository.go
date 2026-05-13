package repository

import (
	"context"
	"encoding/json"
	"errors"

	"diploma/notification-service/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotificationNotFound = errors.New("notification not found")

type NotificationRepository struct {
	db *pgxpool.Pool
}

func NewNotificationRepository(db *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS notifications (
			notification_id BIGSERIAL PRIMARY KEY,
			user_id BIGINT NOT NULL,
			source_service TEXT NOT NULL,
			type TEXT NOT NULL,
			title TEXT NOT NULL,
			body TEXT NOT NULL,
			data JSONB NOT NULL DEFAULT '{}'::jsonb,
			status TEXT NOT NULL DEFAULT 'unread'
				CHECK (status IN ('unread', 'read')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			read_at TIMESTAMPTZ
		)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_created
			ON notifications(user_id, created_at DESC, notification_id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_user_status
			ON notifications(user_id, status)`,
	}

	for _, stmt := range statements {
		if _, err := r.db.Exec(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

func (r *NotificationRepository) Create(
	ctx context.Context,
	userID int64,
	sourceService string,
	notificationType string,
	title string,
	body string,
	data []byte,
) (model.Notification, error) {
	if !json.Valid(data) {
		data = []byte(`{}`)
	}

	var item model.Notification
	err := r.db.QueryRow(ctx, `
		INSERT INTO notifications (
			user_id,
			source_service,
			type,
			title,
			body,
			data
		)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb)
		RETURNING notification_id, user_id, source_service, type, title, body,
			data, status, created_at, read_at
	`, userID, sourceService, notificationType, title, body, string(data)).Scan(
		&item.ID,
		&item.UserID,
		&item.SourceService,
		&item.Type,
		&item.Title,
		&item.Body,
		&item.Data,
		&item.Status,
		&item.CreatedAt,
		&item.ReadAt,
	)
	if err != nil {
		return model.Notification{}, err
	}

	return item, nil
}

func (r *NotificationRepository) ListByUser(
	ctx context.Context,
	userID int64,
	limit int,
	unreadOnly bool,
) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT notification_id, user_id, source_service, type, title, body,
			data, status, created_at, read_at
		FROM notifications
		WHERE user_id = $1
		  AND ($2::BOOLEAN = FALSE OR status = 'unread')
		ORDER BY created_at DESC, notification_id DESC
		LIMIT $3
	`, userID, unreadOnly, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]model.Notification, 0)
	for rows.Next() {
		var item model.Notification
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.SourceService,
			&item.Type,
			&item.Title,
			&item.Body,
			&item.Data,
			&item.Status,
			&item.CreatedAt,
			&item.ReadAt,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (r *NotificationRepository) MarkRead(
	ctx context.Context,
	notificationID int64,
	userID int64,
) (model.Notification, error) {
	var item model.Notification
	err := r.db.QueryRow(ctx, `
		UPDATE notifications
		SET status = 'read',
			read_at = COALESCE(read_at, NOW())
		WHERE notification_id = $1
		  AND user_id = $2
		RETURNING notification_id, user_id, source_service, type, title, body,
			data, status, created_at, read_at
	`, notificationID, userID).Scan(
		&item.ID,
		&item.UserID,
		&item.SourceService,
		&item.Type,
		&item.Title,
		&item.Body,
		&item.Data,
		&item.Status,
		&item.CreatedAt,
		&item.ReadAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Notification{}, ErrNotificationNotFound
	}
	if err != nil {
		return model.Notification{}, err
	}

	return item, nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID int64) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET status = 'read',
			read_at = COALESCE(read_at, NOW())
		WHERE user_id = $1
		  AND status = 'unread'
	`, userID)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}

func (r *NotificationRepository) CountUnread(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM notifications
		WHERE user_id = $1
		  AND status = 'unread'
	`, userID).Scan(&count)
	return count, err
}

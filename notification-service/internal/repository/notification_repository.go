package repository

import (
	"context"
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
	_, err := r.db.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS notifications (
			notification_id SERIAL PRIMARY KEY,
			user_id INTEGER REFERENCES users(user_id) ON DELETE CASCADE,
			type VARCHAR(50),
			title VARCHAR(255),
			message TEXT,
			is_read BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_notifications_user_id_created_at
			ON notifications(user_id, created_at DESC);

		CREATE INDEX IF NOT EXISTS idx_notifications_user_id_is_read
			ON notifications(user_id, is_read);
	`)
	return err
}

func (r *NotificationRepository) Create(
	ctx context.Context,
	userID int,
	notificationType string,
	title string,
	message string,
) (model.Notification, error) {
	var notification model.Notification
	err := r.db.QueryRow(ctx, `
		INSERT INTO notifications (user_id, type, title, message)
		VALUES ($1, $2, $3, $4)
		RETURNING notification_id, user_id, COALESCE(type, ''), COALESCE(title, ''),
			COALESCE(message, ''), is_read, created_at
	`, userID, notificationType, title, message).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&notification.IsRead,
		&notification.CreatedAt,
	)
	if err != nil {
		return model.Notification{}, err
	}

	return notification, nil
}

func (r *NotificationRepository) ListByUser(
	ctx context.Context,
	userID int,
	limit int,
	onlyUnread bool,
) ([]model.Notification, error) {
	rows, err := r.db.Query(ctx, `
		SELECT notification_id, user_id, COALESCE(type, ''), COALESCE(title, ''),
			COALESCE(message, ''), is_read, created_at
		FROM notifications
		WHERE user_id = $1
		  AND ($2 = false OR is_read = false)
		ORDER BY created_at DESC, notification_id DESC
		LIMIT $3
	`, userID, onlyUnread, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	notifications := make([]model.Notification, 0)
	for rows.Next() {
		var notification model.Notification
		if err := rows.Scan(
			&notification.ID,
			&notification.UserID,
			&notification.Type,
			&notification.Title,
			&notification.Message,
			&notification.IsRead,
			&notification.CreatedAt,
		); err != nil {
			return nil, err
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}

func (r *NotificationRepository) MarkRead(
	ctx context.Context,
	notificationID int,
	userID int,
) (model.Notification, error) {
	var notification model.Notification
	err := r.db.QueryRow(ctx, `
		UPDATE notifications
		SET is_read = true
		WHERE notification_id = $1
		  AND user_id = $2
		RETURNING notification_id, user_id, COALESCE(type, ''), COALESCE(title, ''),
			COALESCE(message, ''), is_read, created_at
	`, notificationID, userID).Scan(
		&notification.ID,
		&notification.UserID,
		&notification.Type,
		&notification.Title,
		&notification.Message,
		&notification.IsRead,
		&notification.CreatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Notification{}, ErrNotificationNotFound
	}
	if err != nil {
		return model.Notification{}, err
	}

	return notification, nil
}

func (r *NotificationRepository) MarkAllRead(ctx context.Context, userID int) (int64, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE notifications
		SET is_read = true
		WHERE user_id = $1
		  AND is_read = false
	`, userID)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}

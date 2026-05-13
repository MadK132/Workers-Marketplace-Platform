package service

import (
	"context"
	"errors"
	"strings"

	"diploma/notification-service/internal/model"
	"diploma/notification-service/internal/repository"
)

var (
	ErrInvalidInput         = errors.New("invalid notification input")
	ErrNotificationNotFound = repository.ErrNotificationNotFound
)

type NotificationRepository interface {
	Create(ctx context.Context, userID int, notificationType string, title string, message string) (model.Notification, error)
	ListByUser(ctx context.Context, userID int, limit int, onlyUnread bool) ([]model.Notification, error)
	MarkRead(ctx context.Context, notificationID int, userID int) (model.Notification, error)
	MarkAllRead(ctx context.Context, userID int) (int64, error)
}

type NotificationService struct {
	repo NotificationRepository
}

func NewNotificationService(repo NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) Create(
	ctx context.Context,
	userID int,
	notificationType string,
	title string,
	message string,
) (model.Notification, error) {
	notificationType = strings.TrimSpace(notificationType)
	title = strings.TrimSpace(title)
	message = strings.TrimSpace(message)

	if userID <= 0 || title == "" || message == "" {
		return model.Notification{}, ErrInvalidInput
	}
	if len(notificationType) > 50 || len(title) > 255 {
		return model.Notification{}, ErrInvalidInput
	}

	return s.repo.Create(ctx, userID, notificationType, title, message)
}

func (s *NotificationService) ListByUser(
	ctx context.Context,
	userID int,
	limit int,
	onlyUnread bool,
) ([]model.Notification, error) {
	if userID <= 0 {
		return nil, ErrInvalidInput
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	return s.repo.ListByUser(ctx, userID, limit, onlyUnread)
}

func (s *NotificationService) MarkRead(
	ctx context.Context,
	notificationID int,
	userID int,
) (model.Notification, error) {
	if notificationID <= 0 || userID <= 0 {
		return model.Notification{}, ErrInvalidInput
	}

	return s.repo.MarkRead(ctx, notificationID, userID)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID int) (int64, error) {
	if userID <= 0 {
		return 0, ErrInvalidInput
	}

	return s.repo.MarkAllRead(ctx, userID)
}

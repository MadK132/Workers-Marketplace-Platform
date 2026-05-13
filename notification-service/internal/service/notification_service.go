package service

import (
	"context"
	"errors"

	"diploma/internal/notifications"
	"diploma/notification-service/internal/model"
	"diploma/notification-service/internal/repository"
)

var ErrInvalidNotification = errors.New("invalid notification")

type NotificationService struct {
	repo *repository.NotificationRepository
}

func NewNotificationService(repo *repository.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) CreateFromEvent(
	ctx context.Context,
	event notifications.Event,
) (model.Notification, error) {
	event = event.Normalize()
	if err := event.Validate(); err != nil {
		return model.Notification{}, ErrInvalidNotification
	}

	return s.repo.Create(
		ctx,
		event.RecipientUserID,
		event.SourceService,
		event.Type,
		event.Title,
		event.Body,
		event.DataJSON(),
	)
}

func (s *NotificationService) List(
	ctx context.Context,
	userID int64,
	limit int,
	unreadOnly bool,
) ([]model.Notification, error) {
	if userID <= 0 {
		return nil, ErrInvalidNotification
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	return s.repo.ListByUser(ctx, userID, limit, unreadOnly)
}

func (s *NotificationService) MarkRead(
	ctx context.Context,
	notificationID int64,
	userID int64,
) (model.Notification, error) {
	if notificationID <= 0 || userID <= 0 {
		return model.Notification{}, ErrInvalidNotification
	}

	return s.repo.MarkRead(ctx, notificationID, userID)
}

func (s *NotificationService) MarkAllRead(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, ErrInvalidNotification
	}

	return s.repo.MarkAllRead(ctx, userID)
}

func (s *NotificationService) CountUnread(ctx context.Context, userID int64) (int64, error) {
	if userID <= 0 {
		return 0, ErrInvalidNotification
	}

	return s.repo.CountUnread(ctx, userID)
}

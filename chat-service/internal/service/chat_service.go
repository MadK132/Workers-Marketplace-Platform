package service

import (
	"context"
	"errors"
	"strings"

	"diploma/chat-service/internal/model"
	"diploma/chat-service/internal/repository"
)

var (
	ErrForbidden      = errors.New("user is not allowed to access this chat")
	ErrInvalidInput   = errors.New("invalid chat input")
	ErrMessageTooLong = errors.New("message content is too long")
)

type ChatService struct {
	repo *repository.ChatRepository
}

func NewChatService(repo *repository.ChatRepository) *ChatService {
	return &ChatService{repo: repo}
}

func (s *ChatService) CreateChat(
	ctx context.Context,
	currentUserID int64,
	role string,
	bookingID int64,
	customerUserID int64,
	workerUserID int64,
) (model.Chat, error) {
	if bookingID <= 0 || customerUserID <= 0 || workerUserID <= 0 || customerUserID == workerUserID {
		return model.Chat{}, ErrInvalidInput
	}

	switch role {
	case "admin":
	case "customer":
		if currentUserID != customerUserID {
			return model.Chat{}, ErrForbidden
		}
	case "worker":
		if currentUserID != workerUserID {
			return model.Chat{}, ErrForbidden
		}
	default:
		return model.Chat{}, ErrForbidden
	}

	return s.repo.UpsertChat(ctx, bookingID, customerUserID, workerUserID)
}

func (s *ChatService) ListChats(ctx context.Context, userID int64) ([]model.Chat, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *ChatService) GetChat(ctx context.Context, chatID int64, userID int64) (model.Chat, error) {
	if chatID <= 0 || userID <= 0 {
		return model.Chat{}, ErrInvalidInput
	}

	return s.repo.GetByIDForUser(ctx, chatID, userID)
}

func (s *ChatService) SendMessage(
	ctx context.Context,
	chatID int64,
	senderUserID int64,
	content string,
) (model.Message, error) {
	content = strings.TrimSpace(content)
	if chatID <= 0 || senderUserID <= 0 || content == "" {
		return model.Message{}, ErrInvalidInput
	}
	if len([]rune(content)) > 4000 {
		return model.Message{}, ErrMessageTooLong
	}

	return s.repo.CreateMessage(ctx, chatID, senderUserID, content)
}

func (s *ChatService) ListMessages(
	ctx context.Context,
	chatID int64,
	userID int64,
	limit int,
	beforeID int64,
) ([]model.Message, error) {
	if chatID <= 0 || userID <= 0 {
		return nil, ErrInvalidInput
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if beforeID < 0 {
		beforeID = 0
	}

	return s.repo.ListMessages(ctx, chatID, userID, limit, beforeID)
}

func (s *ChatService) MarkRead(ctx context.Context, chatID int64, userID int64) (int64, error) {
	if chatID <= 0 || userID <= 0 {
		return 0, ErrInvalidInput
	}

	return s.repo.MarkRead(ctx, chatID, userID)
}

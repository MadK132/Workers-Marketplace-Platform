package repository

import (
	"context"
	"errors"

	"diploma/chat-service/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrChatNotFound    = errors.New("chat not found")
	ErrBookingNotFound = errors.New("booking not found")
)

type BookingParticipants struct {
	CustomerUserID int64
	WorkerUserID   int64
}

type ChatRepository struct {
	db *pgxpool.Pool
}

func NewChatRepository(db *pgxpool.Pool) *ChatRepository {
	return &ChatRepository{db: db}
}

func (r *ChatRepository) EnsureSchema(ctx context.Context) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS chats (
			chat_id BIGSERIAL PRIMARY KEY,
			booking_id BIGINT NOT NULL UNIQUE,
			customer_user_id BIGINT NOT NULL,
			worker_user_id BIGINT NOT NULL,
			status TEXT NOT NULL DEFAULT 'active'
				CHECK (status IN ('active', 'closed')),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CHECK (customer_user_id <> worker_user_id)
		)`,
		`CREATE TABLE IF NOT EXISTS chat_messages (
			message_id BIGSERIAL PRIMARY KEY,
			chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
			sender_user_id BIGINT NOT NULL,
			content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000),
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			read_at TIMESTAMPTZ
		)`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS customer_user_id BIGINT`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS worker_user_id BIGINT`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active'`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
		`ALTER TABLE chats ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
		`UPDATE chats c
		 SET customer_user_id = cp.user_id,
		     worker_user_id = wp.user_id
		 FROM bookings b
		 JOIN service_requests sr ON sr.request_id = b.request_id
		 JOIN customer_profiles cp ON cp.customer_profile_id = sr.customer_profile_id
		 JOIN worker_profiles wp ON wp.worker_profile_id = b.worker_profile_id
		 WHERE c.booking_id = b.booking_id
		   AND (c.customer_user_id IS NULL OR c.worker_user_id IS NULL)`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS content TEXT`,
		`DO $$
		BEGIN
			IF EXISTS (
				SELECT 1
				FROM information_schema.columns
				WHERE table_schema = current_schema()
				  AND table_name = 'chat_messages'
				  AND column_name = 'message_text'
			) THEN
				EXECUTE 'UPDATE chat_messages SET content = message_text WHERE content IS NULL';
			END IF;
		END $$`,
		`UPDATE chat_messages SET content = '[migrated empty message]' WHERE content IS NULL`,
		`ALTER TABLE chat_messages ALTER COLUMN content SET NOT NULL`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ`,
		`CREATE INDEX IF NOT EXISTS idx_chats_customer_user_id ON chats(customer_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chats_worker_user_id ON chats(worker_user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_chat_id_message_id
			ON chat_messages(chat_id, message_id DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_chat_messages_unread
			ON chat_messages(chat_id, sender_user_id, read_at)`,
	}

	for _, stmt := range statements {
		if _, err := r.db.Exec(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

func (r *ChatRepository) GetBookingParticipants(
	ctx context.Context,
	bookingID int64,
) (BookingParticipants, error) {
	var participants BookingParticipants
	err := r.db.QueryRow(ctx, `
		SELECT cp.user_id, wp.user_id
		FROM bookings b
		JOIN service_requests sr ON sr.request_id = b.request_id
		JOIN customer_profiles cp ON cp.customer_profile_id = sr.customer_profile_id
		JOIN worker_profiles wp ON wp.worker_profile_id = b.worker_profile_id
		WHERE b.booking_id = $1
	`, bookingID).Scan(
		&participants.CustomerUserID,
		&participants.WorkerUserID,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return BookingParticipants{}, ErrBookingNotFound
	}
	if err != nil {
		return BookingParticipants{}, err
	}

	return participants, nil
}

func (r *ChatRepository) UpsertChat(
	ctx context.Context,
	bookingID int64,
	customerUserID int64,
	workerUserID int64,
) (model.Chat, error) {
	var chat model.Chat
	err := r.db.QueryRow(ctx, `
		INSERT INTO chats (booking_id, customer_user_id, worker_user_id)
		VALUES ($1, $2, $3)
		ON CONFLICT (booking_id) DO UPDATE
		SET updated_at = chats.updated_at
		RETURNING chat_id, booking_id, customer_user_id, worker_user_id,
			status, created_at, updated_at
	`, bookingID, customerUserID, workerUserID).Scan(
		&chat.ChatID,
		&chat.BookingID,
		&chat.CustomerUserID,
		&chat.WorkerUserID,
		&chat.Status,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
	if err != nil {
		return model.Chat{}, err
	}

	return chat, nil
}

func (r *ChatRepository) ListByUser(ctx context.Context, userID int64) ([]model.Chat, error) {
	rows, err := r.db.Query(ctx, `
		SELECT
			c.chat_id,
			c.booking_id,
			c.customer_user_id,
			c.worker_user_id,
			c.status,
			c.created_at,
			c.updated_at,
			(
				SELECT COUNT(*)
				FROM chat_messages m
				WHERE m.chat_id = c.chat_id
				  AND m.sender_user_id <> $1
				  AND m.read_at IS NULL
			) AS unread_count
		FROM chats c
		WHERE c.customer_user_id = $1 OR c.worker_user_id = $1
		ORDER BY c.updated_at DESC, c.chat_id DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chats := make([]model.Chat, 0)
	for rows.Next() {
		var chat model.Chat
		if err := rows.Scan(
			&chat.ChatID,
			&chat.BookingID,
			&chat.CustomerUserID,
			&chat.WorkerUserID,
			&chat.Status,
			&chat.CreatedAt,
			&chat.UpdatedAt,
			&chat.UnreadCount,
		); err != nil {
			return nil, err
		}
		chats = append(chats, chat)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return chats, nil
}

func (r *ChatRepository) GetByIDForUser(
	ctx context.Context,
	chatID int64,
	userID int64,
) (model.Chat, error) {
	var chat model.Chat
	err := r.db.QueryRow(ctx, `
		SELECT chat_id, booking_id, customer_user_id, worker_user_id,
			status, created_at, updated_at
		FROM chats
		WHERE chat_id = $1
		  AND (customer_user_id = $2 OR worker_user_id = $2)
	`, chatID, userID).Scan(
		&chat.ChatID,
		&chat.BookingID,
		&chat.CustomerUserID,
		&chat.WorkerUserID,
		&chat.Status,
		&chat.CreatedAt,
		&chat.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Chat{}, ErrChatNotFound
	}
	if err != nil {
		return model.Chat{}, err
	}

	return chat, nil
}

func (r *ChatRepository) CreateMessage(
	ctx context.Context,
	chatID int64,
	senderUserID int64,
	content string,
) (model.Message, error) {
	var msg model.Message
	err := r.db.QueryRow(ctx, `
		WITH allowed_chat AS (
			SELECT chat_id
			FROM chats
			WHERE chat_id = $1
			  AND status = 'active'
			  AND (customer_user_id = $2 OR worker_user_id = $2)
		), inserted AS (
			INSERT INTO chat_messages (chat_id, sender_user_id, content)
			SELECT chat_id, $2, $3
			FROM allowed_chat
			RETURNING message_id, chat_id, sender_user_id, content, created_at, read_at
		), touched AS (
			UPDATE chats
			SET updated_at = NOW()
			WHERE chat_id IN (SELECT chat_id FROM inserted)
		)
		SELECT message_id, chat_id, sender_user_id, content, created_at, read_at
		FROM inserted
	`, chatID, senderUserID, content).Scan(
		&msg.MessageID,
		&msg.ChatID,
		&msg.SenderUserID,
		&msg.Content,
		&msg.CreatedAt,
		&msg.ReadAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Message{}, ErrChatNotFound
	}
	if err != nil {
		return model.Message{}, err
	}

	return msg, nil
}

func (r *ChatRepository) ListMessages(
	ctx context.Context,
	chatID int64,
	userID int64,
	limit int,
	beforeID int64,
) ([]model.Message, error) {
	rows, err := r.db.Query(ctx, `
		SELECT m.message_id, m.chat_id, m.sender_user_id, m.content,
			m.created_at, m.read_at
		FROM chat_messages m
		JOIN chats c ON c.chat_id = m.chat_id
		WHERE m.chat_id = $1
		  AND (c.customer_user_id = $2 OR c.worker_user_id = $2)
		  AND ($3::BIGINT = 0 OR m.message_id < $3)
		ORDER BY m.message_id DESC
		LIMIT $4
	`, chatID, userID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := make([]model.Message, 0)
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(
			&msg.MessageID,
			&msg.ChatID,
			&msg.SenderUserID,
			&msg.Content,
			&msg.CreatedAt,
			&msg.ReadAt,
		); err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i, j := 0, len(messages)-1; i < j; i, j = i+1, j-1 {
		messages[i], messages[j] = messages[j], messages[i]
	}

	return messages, nil
}

func (r *ChatRepository) MarkRead(
	ctx context.Context,
	chatID int64,
	userID int64,
) (int64, error) {
	if _, err := r.GetByIDForUser(ctx, chatID, userID); err != nil {
		return 0, err
	}

	tag, err := r.db.Exec(ctx, `
		UPDATE chat_messages
		SET read_at = NOW()
		WHERE chat_id = $1
		  AND sender_user_id <> $2
		  AND read_at IS NULL
	`, chatID, userID)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}

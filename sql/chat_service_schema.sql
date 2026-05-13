CREATE TABLE IF NOT EXISTS chats (
  chat_id BIGSERIAL PRIMARY KEY,
  booking_id BIGINT NOT NULL UNIQUE,
  customer_user_id BIGINT NOT NULL,
  worker_user_id BIGINT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active'
    CHECK (status IN ('active', 'closed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK (customer_user_id <> worker_user_id)
);

CREATE TABLE IF NOT EXISTS chat_messages (
  message_id BIGSERIAL PRIMARY KEY,
  chat_id BIGINT NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
  sender_user_id BIGINT NOT NULL,
  content TEXT NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  read_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_chats_customer_user_id
  ON chats(customer_user_id);

CREATE INDEX IF NOT EXISTS idx_chats_worker_user_id
  ON chats(worker_user_id);

CREATE INDEX IF NOT EXISTS idx_chat_messages_chat_id_message_id
  ON chat_messages(chat_id, message_id DESC);

CREATE INDEX IF NOT EXISTS idx_chat_messages_unread
  ON chat_messages(chat_id, sender_user_id, read_at);

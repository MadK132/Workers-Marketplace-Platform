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

ALTER TABLE chats ADD COLUMN IF NOT EXISTS customer_user_id BIGINT;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS worker_user_id BIGINT;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'active';
ALTER TABLE chats ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE chats ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

UPDATE chats c
SET customer_user_id = cp.user_id,
    worker_user_id = wp.user_id
FROM bookings b
JOIN service_requests sr ON sr.request_id = b.request_id
JOIN customer_profiles cp ON cp.customer_profile_id = sr.customer_profile_id
JOIN worker_profiles wp ON wp.worker_profile_id = b.worker_profile_id
WHERE c.booking_id = b.booking_id
  AND (c.customer_user_id IS NULL OR c.worker_user_id IS NULL);

ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS content TEXT;

DO $$
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
END $$;

UPDATE chat_messages
SET content = '[migrated empty message]'
WHERE content IS NULL;

ALTER TABLE chat_messages ALTER COLUMN content SET NOT NULL;
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS read_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_chats_customer_user_id
  ON chats(customer_user_id);

CREATE INDEX IF NOT EXISTS idx_chats_worker_user_id
  ON chats(worker_user_id);

CREATE INDEX IF NOT EXISTS idx_chat_messages_chat_id_message_id
  ON chat_messages(chat_id, message_id DESC);

CREATE INDEX IF NOT EXISTS idx_chat_messages_unread
  ON chat_messages(chat_id, sender_user_id, read_at);

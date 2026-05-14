-- Portable schema for Workers Marketplace.
-- Run on an empty PostgreSQL database with PostGIS installed in the container/image.
-- Example:
--   docker exec -i postgres psql -U user -d app < sql/schema_for_friend.sql

BEGIN;

CREATE EXTENSION IF NOT EXISTS postgis;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'booking_status') THEN
    CREATE TYPE booking_status AS ENUM ('scheduled', 'in_progress', 'completed', 'cancelled');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'experience_level') THEN
    CREATE TYPE experience_level AS ENUM ('junior', 'middle', 'senior');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'payment_status') THEN
    CREATE TYPE payment_status AS ENUM ('pending', 'completed', 'failed', 'refunded');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'request_status') THEN
    CREATE TYPE request_status AS ENUM ('pending', 'accepted', 'in_progress', 'completed', 'cancelled');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_role') THEN
    CREATE TYPE user_role AS ENUM ('customer', 'worker', 'admin');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'user_status') THEN
    CREATE TYPE user_status AS ENUM ('active', 'inactive', 'banned');
  END IF;

  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'verification_status') THEN
    CREATE TYPE verification_status AS ENUM ('unverified', 'pending', 'verified', 'rejected');
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS users (
  user_id serial PRIMARY KEY,
  full_name varchar(255) NOT NULL,
  email varchar(255) NOT NULL UNIQUE,
  phone varchar(50) UNIQUE,
  password_hash varchar(255) NOT NULL,
  role user_role,
  status user_status DEFAULT 'inactive',
  created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS customer_profiles (
  customer_profile_id serial PRIMARY KEY,
  user_id integer UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
  address text,
  latitude numeric(9, 6),
  longitude numeric(9, 6),
  location geography(Point, 4326)
);

CREATE TABLE IF NOT EXISTS worker_profiles (
  worker_profile_id serial PRIMARY KEY,
  user_id integer UNIQUE REFERENCES users(user_id) ON DELETE CASCADE,
  bio text,
  rating numeric(3, 2) CHECK (rating >= 0 AND rating <= 5),
  verification_status verification_status DEFAULT 'unverified',
  current_latitude numeric(9, 6),
  current_longitude numeric(9, 6),
  is_available boolean DEFAULT false,
  profile_photo_url text,
  current_location geography(Point, 4326)
);

CREATE TABLE IF NOT EXISTS service_categories (
  category_id serial PRIMARY KEY,
  name varchar(100) NOT NULL,
  description text
);

CREATE TABLE IF NOT EXISTS service_requests (
  request_id serial PRIMARY KEY,
  customer_profile_id integer REFERENCES customer_profiles(customer_profile_id) ON DELETE RESTRICT,
  category_id integer REFERENCES service_categories(category_id) ON DELETE RESTRICT,
  description text,
  address text,
  latitude numeric(9, 6),
  longitude numeric(9, 6),
  status request_status DEFAULT 'pending',
  created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
  location geography(Point, 4326)
);

CREATE TABLE IF NOT EXISTS worker_skills (
  worker_skill_id serial PRIMARY KEY,
  worker_profile_id integer REFERENCES worker_profiles(worker_profile_id) ON DELETE CASCADE,
  category_id integer REFERENCES service_categories(category_id) ON DELETE CASCADE,
  experience_level experience_level NOT NULL,
  price_base numeric(10, 2),
  is_verified boolean DEFAULT false,
  CONSTRAINT unique_worker_category UNIQUE (worker_profile_id, category_id)
);

CREATE TABLE IF NOT EXISTS worker_skill_evidence (
  evidence_id serial PRIMARY KEY,
  worker_skill_id integer NOT NULL REFERENCES worker_skills(worker_skill_id) ON DELETE CASCADE,
  file_name varchar(255) NOT NULL,
  file_path text NOT NULL,
  content_type varchar(100),
  note text,
  created_at timestamp with time zone DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bookings (
  booking_id serial PRIMARY KEY,
  request_id integer REFERENCES service_requests(request_id) ON DELETE RESTRICT,
  worker_profile_id integer REFERENCES worker_profiles(worker_profile_id) ON DELETE RESTRICT,
  scheduled_time timestamp without time zone,
  start_time timestamp without time zone,
  end_time timestamp without time zone,
  status booking_status DEFAULT 'scheduled',
  final_price numeric(10, 2)
);

CREATE TABLE IF NOT EXISTS payments (
  payment_id serial PRIMARY KEY,
  booking_id integer UNIQUE REFERENCES bookings(booking_id) ON DELETE RESTRICT,
  amount numeric(10, 2),
  currency char(3) DEFAULT 'USD',
  payment_status payment_status DEFAULT 'pending',
  payment_method varchar(50),
  transaction_reference varchar(255),
  paid_at timestamp without time zone
);

CREATE TABLE IF NOT EXISTS reviews (
  review_id serial PRIMARY KEY,
  booking_id integer UNIQUE REFERENCES bookings(booking_id) ON DELETE CASCADE,
  customer_profile_id integer REFERENCES customer_profiles(customer_profile_id) ON DELETE SET NULL,
  worker_profile_id integer REFERENCES worker_profiles(worker_profile_id) ON DELETE SET NULL,
  rating integer CHECK (rating >= 1 AND rating <= 5),
  comment text,
  created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS chats (
  chat_id bigserial PRIMARY KEY,
  booking_id bigint NOT NULL UNIQUE REFERENCES bookings(booking_id) ON DELETE CASCADE,
  customer_user_id bigint NOT NULL,
  worker_user_id bigint NOT NULL,
  status text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'closed')),
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  updated_at timestamp with time zone NOT NULL DEFAULT now(),
  CHECK (customer_user_id <> worker_user_id)
);

CREATE TABLE IF NOT EXISTS chat_messages (
  message_id bigserial PRIMARY KEY,
  chat_id bigint NOT NULL REFERENCES chats(chat_id) ON DELETE CASCADE,
  sender_user_id bigint NOT NULL,
  content text NOT NULL CHECK (char_length(content) BETWEEN 1 AND 4000),
  created_at timestamp with time zone NOT NULL DEFAULT now(),
  read_at timestamp with time zone
);

ALTER TABLE chats ADD COLUMN IF NOT EXISTS customer_user_id bigint;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS worker_user_id bigint;
ALTER TABLE chats ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active';
ALTER TABLE chats ADD COLUMN IF NOT EXISTS created_at timestamp with time zone NOT NULL DEFAULT now();
ALTER TABLE chats ADD COLUMN IF NOT EXISTS updated_at timestamp with time zone NOT NULL DEFAULT now();

UPDATE chats c
SET customer_user_id = cp.user_id,
    worker_user_id = wp.user_id
FROM bookings b
JOIN service_requests sr ON sr.request_id = b.request_id
JOIN customer_profiles cp ON cp.customer_profile_id = sr.customer_profile_id
JOIN worker_profiles wp ON wp.worker_profile_id = b.worker_profile_id
WHERE c.booking_id = b.booking_id
  AND (c.customer_user_id IS NULL OR c.worker_user_id IS NULL);

ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS content text;

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
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS created_at timestamp with time zone NOT NULL DEFAULT now();
ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS read_at timestamp with time zone;

CREATE TABLE IF NOT EXISTS notifications (
  notification_id serial PRIMARY KEY,
  user_id integer REFERENCES users(user_id) ON DELETE CASCADE,
  type varchar(50),
  title varchar(255),
  message text,
  is_read boolean DEFAULT false,
  created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS email_verifications (
  id serial PRIMARY KEY,
  user_id integer REFERENCES users(user_id) ON DELETE CASCADE,
  token text NOT NULL UNIQUE,
  expires_at timestamp without time zone NOT NULL
);

CREATE TABLE IF NOT EXISTS password_resets (
  id serial PRIMARY KEY,
  user_id integer REFERENCES users(user_id) ON DELETE CASCADE,
  token text NOT NULL UNIQUE,
  expires_at timestamp without time zone NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_bookings_worker
  ON bookings(worker_profile_id);

CREATE INDEX IF NOT EXISTS idx_customer_location
  ON customer_profiles(latitude, longitude);

CREATE INDEX IF NOT EXISTS idx_customer_profiles_location
  ON customer_profiles USING gist(location);

CREATE INDEX IF NOT EXISTS idx_chats_customer_user_id
  ON chats(customer_user_id);

CREATE INDEX IF NOT EXISTS idx_chats_worker_user_id
  ON chats(worker_user_id);

CREATE INDEX IF NOT EXISTS idx_chat_messages_chat_id_message_id
  ON chat_messages(chat_id, message_id DESC);

CREATE INDEX IF NOT EXISTS idx_chat_messages_unread
  ON chat_messages(chat_id, sender_user_id, read_at);

CREATE INDEX IF NOT EXISTS idx_messages_chat_id
  ON chat_messages(chat_id);

CREATE INDEX IF NOT EXISTS idx_messages_sender
  ON chat_messages(sender_user_id);

CREATE INDEX IF NOT EXISTS idx_requests_category
  ON service_requests(category_id);

CREATE INDEX IF NOT EXISTS idx_service_requests_location
  ON service_requests USING gist(location);

CREATE INDEX IF NOT EXISTS idx_worker_location
  ON worker_profiles(current_latitude, current_longitude);

CREATE INDEX IF NOT EXISTS idx_worker_profiles_current_location
  ON worker_profiles USING gist(current_location);

CREATE INDEX IF NOT EXISTS idx_worker_skill_evidence_skill_id
  ON worker_skill_evidence(worker_skill_id);

COMMIT;

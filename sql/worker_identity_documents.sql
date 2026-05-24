CREATE TABLE IF NOT EXISTS worker_identity_documents (
  identity_document_id serial PRIMARY KEY,
  worker_profile_id integer NOT NULL REFERENCES worker_profiles(worker_profile_id) ON DELETE CASCADE,
  file_name varchar(255) NOT NULL,
  file_path text NOT NULL,
  content_type varchar(100),
  status varchar(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'verified', 'rejected', 'replaced')),
  rejection_reason text,
  created_at timestamp with time zone DEFAULT now(),
  reviewed_at timestamp with time zone,
  reviewed_by_user_id integer REFERENCES users(user_id) ON DELETE SET NULL,
  assigned_manager_id integer REFERENCES users(user_id) ON DELETE SET NULL
);

ALTER TABLE worker_identity_documents
  ADD COLUMN IF NOT EXISTS assigned_manager_id integer REFERENCES users(user_id) ON DELETE SET NULL;

ALTER TABLE worker_identity_documents
  DROP CONSTRAINT IF EXISTS worker_identity_documents_status_check;

ALTER TABLE worker_identity_documents
  ADD CONSTRAINT worker_identity_documents_status_check
  CHECK (status IN ('pending', 'verified', 'rejected', 'replaced'));

CREATE INDEX IF NOT EXISTS idx_worker_identity_documents_profile_status
  ON worker_identity_documents(worker_profile_id, status, created_at DESC);

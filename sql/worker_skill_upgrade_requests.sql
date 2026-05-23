CREATE TABLE IF NOT EXISTS worker_skill_upgrade_requests (
  upgrade_request_id serial PRIMARY KEY,
  worker_skill_id integer NOT NULL REFERENCES worker_skills(worker_skill_id) ON DELETE CASCADE,
  worker_profile_id integer NOT NULL REFERENCES worker_profiles(worker_profile_id) ON DELETE CASCADE,
  requested_experience_level experience_level NOT NULL,
  evidence_files text NOT NULL DEFAULT '',
  admin_note text NOT NULL DEFAULT '',
  status varchar(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'approved', 'rejected')),
  created_at timestamp with time zone DEFAULT now(),
  reviewed_at timestamp with time zone,
  reviewed_by_user_id integer REFERENCES users(user_id) ON DELETE SET NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_worker_skill_upgrade_pending_unique
  ON worker_skill_upgrade_requests(worker_skill_id)
  WHERE status = 'pending';

CREATE INDEX IF NOT EXISTS idx_worker_skill_upgrade_status
  ON worker_skill_upgrade_requests(status, created_at DESC);

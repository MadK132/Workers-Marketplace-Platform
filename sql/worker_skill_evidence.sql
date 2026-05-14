CREATE TABLE IF NOT EXISTS worker_skill_evidence (
  evidence_id serial PRIMARY KEY,
  worker_skill_id integer NOT NULL REFERENCES worker_skills(worker_skill_id) ON DELETE CASCADE,
  file_name varchar(255) NOT NULL,
  file_path text NOT NULL,
  content_type varchar(100),
  note text,
  created_at timestamp with time zone DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_worker_skill_evidence_skill_id
  ON worker_skill_evidence(worker_skill_id);

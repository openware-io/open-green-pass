-- Environment registration is a user-confirmed execution baseline, not an AI
-- guess. Keep only non-sensitive requirement/evidence metadata here. Secrets
-- remain in an external secret manager and are referenced by name only.
ALTER TABLE env_runtime
  ADD COLUMN requirements JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN confirmation JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN confirmed_at TIMESTAMPTZ;

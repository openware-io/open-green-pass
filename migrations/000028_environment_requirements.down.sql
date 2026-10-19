ALTER TABLE env_runtime
  DROP COLUMN IF EXISTS confirmed_at,
  DROP COLUMN IF EXISTS confirmation,
  DROP COLUMN IF EXISTS requirements;

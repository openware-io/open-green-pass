DROP INDEX IF EXISTS idx_gen_batch_service;
ALTER TABLE gen_batch DROP COLUMN IF EXISTS source_path;
ALTER TABLE gen_batch DROP COLUMN IF EXISTS service_id;

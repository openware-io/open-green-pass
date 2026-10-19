DROP INDEX IF EXISTS idx_gen_seed_batch_case_key;
ALTER TABLE gen_seed DROP COLUMN IF EXISTS case_key;

DROP INDEX IF EXISTS idx_cov_cell_module;
ALTER TABLE cov_cell DROP COLUMN IF EXISTS module;

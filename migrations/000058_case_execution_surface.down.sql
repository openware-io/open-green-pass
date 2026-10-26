ALTER TABLE cas_case DROP CONSTRAINT IF EXISTS cas_case_execution_surface_check;
DROP INDEX IF EXISTS idx_cas_case_surface;
ALTER TABLE cas_case DROP COLUMN IF EXISTS execution_surface;

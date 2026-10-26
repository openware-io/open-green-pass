DROP INDEX IF EXISTS idx_cas_case_repo;
ALTER TABLE cas_case DROP COLUMN IF EXISTS repo_id;

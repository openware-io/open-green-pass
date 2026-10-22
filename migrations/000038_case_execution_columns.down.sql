DROP INDEX IF EXISTS idx_cas_case_coverage;
DROP INDEX IF EXISTS idx_run_case_result_finding;
ALTER TABLE run_case_result DROP CONSTRAINT IF EXISTS ck_run_case_result_failure_class;
ALTER TABLE run_case_result
  DROP COLUMN IF EXISTS finding_id,
  DROP COLUMN IF EXISTS failure_class;
ALTER TABLE cas_case
  DROP COLUMN IF EXISTS source_snapshot_sha,
  DROP COLUMN IF EXISTS oracle_source,
  DROP COLUMN IF EXISTS coverage_cell_ids;

-- M0: execution/case columns needed by the test engine. Greenfield data; direct ALTER allowed.
ALTER TABLE cas_case
  ADD COLUMN coverage_cell_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN oracle_source TEXT NOT NULL DEFAULT '',
  ADD COLUMN source_snapshot_sha TEXT NOT NULL DEFAULT '';

ALTER TABLE run_case_result
  ADD COLUMN failure_class TEXT,
  ADD COLUMN finding_id BIGINT;

ALTER TABLE run_case_result
  ADD CONSTRAINT ck_run_case_result_failure_class
  CHECK (failure_class IS NULL OR failure_class IN ('product','case','environment','data','expected_change','flaky'));

CREATE INDEX idx_run_case_result_finding ON run_case_result(team_id, finding_id)
  WHERE finding_id IS NOT NULL;
CREATE INDEX idx_cas_case_coverage ON cas_case USING GIN (coverage_cell_ids);

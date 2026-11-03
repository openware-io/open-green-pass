-- Roll back the run/unit observability projection.
-- The expanded states must be converged BEFORE restoring the pre-000061 CHECK
-- constraints; otherwise a live database would fail the down migration.
--   run waiting_review  -> running (work is not finished yet)
--   unit waiting_review -> running
--   unit rejected       -> failed (old schema has no rejected unit state)
UPDATE gen_run SET state='running' WHERE state='waiting_review';
UPDATE gen_run_unit SET state='running' WHERE state='waiting_review';
UPDATE gen_run_unit SET state='failed' WHERE state='rejected';

DROP FUNCTION IF EXISTS gp.generation_reconcile_candidates(INTEGER);
DROP INDEX IF EXISTS idx_gen_run_reconcile;

ALTER TABLE gen_run DROP CONSTRAINT IF EXISTS gen_run_state_check;
ALTER TABLE gen_run ADD CONSTRAINT gen_run_state_check
  CHECK (state IN ('queued','running','succeeded','partial_failed','failed','cancelled'));
ALTER TABLE gen_run
  DROP COLUMN IF EXISTS plan_id,
  DROP COLUMN IF EXISTS plan_hash,
  DROP COLUMN IF EXISTS execution_manifest,
  DROP COLUMN IF EXISTS waiting_review_units,
  DROP COLUMN IF EXISTS started_at,
  DROP COLUMN IF EXISTS finished_at,
  DROP COLUMN IF EXISTS cancelled_at,
  DROP COLUMN IF EXISTS cancelled_by;

ALTER TABLE gen_run_unit DROP CONSTRAINT IF EXISTS gen_run_unit_state_check;
ALTER TABLE gen_run_unit ADD CONSTRAINT gen_run_unit_state_check
  CHECK (state IN ('queued','running','succeeded','failed','cancelled'));
ALTER TABLE gen_run_unit
  DROP COLUMN IF EXISTS source_path,
  DROP COLUMN IF EXISTS ref_type,
  DROP COLUMN IF EXISTS ref_name,
  DROP COLUMN IF EXISTS commit_sha,
  DROP COLUMN IF EXISTS batch_state,
  DROP COLUMN IF EXISTS stage,
  DROP COLUMN IF EXISTS review_required,
  DROP COLUMN IF EXISTS started_at,
  DROP COLUMN IF EXISTS finished_at,
  DROP COLUMN IF EXISTS last_synced_at;

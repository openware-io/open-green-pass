-- Run/unit observability and authoritative state (AI domain).
-- gen_run stays the aggregate and gen_batch stays the execution state machine;
-- these columns are the persisted projection the reconciler converges.
ALTER TABLE gen_run
  ADD COLUMN IF NOT EXISTS plan_id BIGINT,
  ADD COLUMN IF NOT EXISTS plan_hash TEXT,
  ADD COLUMN IF NOT EXISTS execution_manifest JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN IF NOT EXISTS waiting_review_units INT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS cancelled_by BIGINT;

ALTER TABLE gen_run DROP CONSTRAINT IF EXISTS gen_run_state_check;
-- NOT VALID keeps the migration non-blocking for existing rows; the application
-- only ever writes the expanded state set.
ALTER TABLE gen_run ADD CONSTRAINT gen_run_state_check
  CHECK (state IN ('queued','running','waiting_review','succeeded','partial_failed','failed','cancelled')) NOT VALID;
CREATE INDEX IF NOT EXISTS idx_gen_run_reconcile ON gen_run(state, updated_at) WHERE state NOT IN ('succeeded','partial_failed','failed','cancelled');

ALTER TABLE gen_run_unit
  ADD COLUMN IF NOT EXISTS source_path TEXT,
  ADD COLUMN IF NOT EXISTS ref_type TEXT,
  ADD COLUMN IF NOT EXISTS ref_name TEXT,
  ADD COLUMN IF NOT EXISTS commit_sha TEXT,
  ADD COLUMN IF NOT EXISTS batch_state TEXT,
  ADD COLUMN IF NOT EXISTS stage TEXT,
  ADD COLUMN IF NOT EXISTS review_required BOOLEAN NOT NULL DEFAULT false,
  ADD COLUMN IF NOT EXISTS started_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS finished_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ;

ALTER TABLE gen_run_unit DROP CONSTRAINT IF EXISTS gen_run_unit_state_check;
ALTER TABLE gen_run_unit ADD CONSTRAINT gen_run_unit_state_check
  CHECK (state IN ('queued','running','waiting_review','succeeded','failed','rejected','cancelled')) NOT VALID;

-- The worker reconciler has no request tenant, and gen_run has RLS enabled.
-- This narrowly scoped definer read returns only run ids that still need
-- convergence; the actual projection writes stay inside tenant-scoped
-- transactions (RLS + team_id filter).
CREATE OR REPLACE FUNCTION gp.generation_reconcile_candidates(p_limit INTEGER DEFAULT 100)
RETURNS TABLE(run_id BIGINT, team_id BIGINT)
LANGUAGE sql
STABLE
SECURITY DEFINER
SET search_path = pg_catalog, gp
AS $$
  SELECT r.id, r.team_id
  FROM gp.gen_run r
  WHERE r.state NOT IN ('succeeded','partial_failed','failed','cancelled')
  ORDER BY r.updated_at, r.id
  LIMIT GREATEST(COALESCE(p_limit, 100), 1)
$$;
REVOKE ALL ON FUNCTION gp.generation_reconcile_candidates(INTEGER) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION gp.generation_reconcile_candidates(INTEGER) TO gp_worker, gp_server;

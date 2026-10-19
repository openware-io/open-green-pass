DROP INDEX IF EXISTS idx_gen_batch_baseline;
ALTER TABLE gen_batch DROP COLUMN IF EXISTS base_batch_id;
ALTER TABLE gen_batch DROP COLUMN IF EXISTS ref_type;
DROP POLICY IF EXISTS p_repo_ref_rls ON repo_ref;
DROP TRIGGER IF EXISTS trg_repo_ref_tenant ON repo_ref;
DROP TRIGGER IF EXISTS trg_repo_ref_audit ON repo_ref;
DROP TABLE IF EXISTS repo_ref;

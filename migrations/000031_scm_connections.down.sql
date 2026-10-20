DROP INDEX IF EXISTS idx_repo_repo_scm_connection;
ALTER TABLE repo_repo DROP COLUMN IF EXISTS scm_connection_id;
DROP POLICY IF EXISTS p_repo_scm_connection_rls ON repo_scm_connection;
DROP TRIGGER IF EXISTS trg_repo_scm_connection_tenant ON repo_scm_connection;
DROP TRIGGER IF EXISTS trg_repo_scm_connection_audit ON repo_scm_connection;
DROP TABLE IF EXISTS repo_scm_connection;

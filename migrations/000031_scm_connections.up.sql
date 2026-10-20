-- 团队级源码管理连接：凭据只保存在 Kubernetes Secret，数据库只保存引用。
CREATE TABLE repo_scm_connection (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  provider TEXT NOT NULL CHECK (provider IN ('github')),
  name TEXT NOT NULL CHECK (btrim(name) <> ''),
  secret_ref TEXT NOT NULL CHECK (btrim(secret_ref) <> ''),
  credential_status TEXT NOT NULL DEFAULT 'configured'
    CHECK (credential_status IN ('configured','verified','failed')),
  failure_summary TEXT NOT NULL DEFAULT '',
  verified_at TIMESTAMPTZ,
  created_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, provider, name)
);
CREATE INDEX idx_repo_scm_connection_team_status ON repo_scm_connection(team_id, provider, credential_status);
CREATE TRIGGER trg_repo_scm_connection_tenant BEFORE INSERT ON repo_scm_connection FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_repo_scm_connection_audit BEFORE UPDATE ON repo_scm_connection FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE repo_scm_connection ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_repo_scm_connection_rls ON repo_scm_connection
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

ALTER TABLE repo_repo
  ADD COLUMN scm_connection_id BIGINT REFERENCES repo_scm_connection(id);
CREATE INDEX idx_repo_repo_scm_connection ON repo_repo(team_id, scm_connection_id)
  WHERE scm_connection_id IS NOT NULL;

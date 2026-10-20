-- GitHub 安装跳转的短时、一次性 state，绑定发起授权的团队和操作者。
CREATE TABLE repo_scm_authorization_state (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  actor_id BIGINT NOT NULL,
  installation_id BIGINT NOT NULL,
  state_hash TEXT NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_repo_scm_authorization_state_expiry ON repo_scm_authorization_state(expires_at);
CREATE TRIGGER trg_repo_scm_authorization_state_tenant BEFORE INSERT ON repo_scm_authorization_state FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE repo_scm_authorization_state ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_repo_scm_authorization_state_rls ON repo_scm_authorization_state
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

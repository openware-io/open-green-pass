-- GitHub App 是正式授权模式：数据库仅保存安装标识与私钥 Secret 引用，
-- 运行时按需换取短时 installation token。
ALTER TABLE repo_scm_connection
  ADD COLUMN auth_type TEXT NOT NULL DEFAULT 'pat' CHECK (auth_type IN ('pat','github_app')),
  ADD COLUMN github_app_id TEXT,
  ADD COLUMN github_installation_id BIGINT,
  ADD COLUMN installation_account_login TEXT NOT NULL DEFAULT '';
CREATE UNIQUE INDEX uk_repo_scm_connection_github_installation
  ON repo_scm_connection(team_id, provider, github_installation_id)
  WHERE github_installation_id IS NOT NULL;

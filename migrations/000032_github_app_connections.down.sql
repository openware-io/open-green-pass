DROP INDEX IF EXISTS uk_repo_scm_connection_github_installation;
ALTER TABLE repo_scm_connection
  DROP COLUMN IF EXISTS installation_account_login,
  DROP COLUMN IF EXISTS github_installation_id,
  DROP COLUMN IF EXISTS github_app_id,
  DROP COLUMN IF EXISTS auth_type;

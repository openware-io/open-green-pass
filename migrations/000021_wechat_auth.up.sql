CREATE TABLE auth_provider_config (
  team_id        BIGINT NOT NULL,
  provider       TEXT NOT NULL CHECK (provider IN ('wechat')),
  enabled        BOOLEAN NOT NULL DEFAULT false,
  app_id         TEXT NOT NULL DEFAULT '',
  callback_uri   TEXT NOT NULL DEFAULT '',
  secret_ref     TEXT NOT NULL DEFAULT '',
  updated_by     BIGINT,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (team_id, provider)
);
CREATE TRIGGER trg_auth_provider_config_tenant BEFORE INSERT ON auth_provider_config FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_auth_provider_config_audit BEFORE UPDATE ON auth_provider_config FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE auth_provider_config ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_auth_provider_config_rls ON auth_provider_config
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE auth_external_identity (
  provider       TEXT NOT NULL CHECK (provider IN ('wechat')),
  external_id    TEXT NOT NULL,
  union_id       TEXT NOT NULL DEFAULT '',
  user_id        BIGINT NOT NULL REFERENCES auth_user(id) ON DELETE CASCADE,
  display_name   TEXT NOT NULL DEFAULT '',
  avatar_url     TEXT NOT NULL DEFAULT '',
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (provider, external_id),
  UNIQUE (provider, user_id)
);

CREATE TABLE auth_oauth_state (
  state_hash     BYTEA PRIMARY KEY,
  team_id        BIGINT NOT NULL,
  user_id        BIGINT,
  purpose        TEXT NOT NULL CHECK (purpose IN ('login','bind')),
  expires_at     TIMESTAMPTZ NOT NULL,
  consumed_at    TIMESTAMPTZ,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_auth_oauth_state_expiry ON auth_oauth_state(expires_at) WHERE consumed_at IS NULL;

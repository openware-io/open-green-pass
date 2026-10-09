-- GP4-03: first-party accounts and revocable browser sessions.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE auth_user (
  id            BIGINT PRIMARY KEY,
  username      TEXT NOT NULL UNIQUE CHECK (username = lower(btrim(username))),
  password_hash TEXT NOT NULL CHECK (password_hash <> ''),
  display_name  TEXT NOT NULL CHECK (btrim(display_name) <> ''),
  email         TEXT NOT NULL DEFAULT '',
  status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE auth_session (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  token_hash  BYTEA NOT NULL UNIQUE,
  user_id     BIGINT NOT NULL REFERENCES auth_user(id) ON DELETE CASCADE,
  team_id     BIGINT NOT NULL,
  expires_at  TIMESTAMPTZ NOT NULL,
  revoked_at  TIMESTAMPTZ,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_auth_session_user_active ON auth_session(user_id, expires_at) WHERE revoked_at IS NULL;

-- Local kind acceptance account. The password is GreenPass@123 and is stored
-- only as a bcrypt digest. Production provisioning must replace this seed.
INSERT INTO auth_user (id, username, password_hash, display_name, email)
VALUES (42, 'zhangli', crypt('GreenPass@123', gen_salt('bf', 12)), '张立', 'zhang.li@corp.com')
ON CONFLICT (id) DO NOTHING;

SELECT set_config('gp.team_id', '1001', true);
INSERT INTO iam_member (id, team_id, principal_id, role, status, created_by, updated_by)
VALUES (42, 1001, '42', 'owner', 'active', 42, 42)
ON CONFLICT (team_id, principal_id) DO UPDATE SET role='owner', status='active', updated_at=now();


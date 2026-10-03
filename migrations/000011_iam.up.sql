-- GP4-01A: team membership and asset-level authorization state.
-- Provider identity claims are deliberately absent; iam_member stores only a
-- stable principal id. All tables are tenant-scoped and protected by RLS.

CREATE TABLE iam_member (
  id             BIGINT PRIMARY KEY,
  team_id        BIGINT NOT NULL,
  principal_id   TEXT NOT NULL CHECK (btrim(principal_id) <> ''),
  role           TEXT NOT NULL CHECK (role IN ('owner','admin','tester','viewer')),
  status         TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled','invited')),
  last_active_at TIMESTAMPTZ,
  created_by     BIGINT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by     BIGINT,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, principal_id)
);
CREATE INDEX idx_iam_member_team_status ON iam_member(team_id, status);
CREATE TRIGGER trg_iam_member_tenant BEFORE INSERT ON iam_member FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_iam_member_audit BEFORE UPDATE ON iam_member FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE iam_member ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_iam_member_rls ON iam_member
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE iam_asset_owner (
  team_id      BIGINT NOT NULL,
  target_id    BIGINT NOT NULL,
  member_id    BIGINT NOT NULL,
  assigned_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  assigned_by  BIGINT,
  PRIMARY KEY (team_id, target_id)
);
CREATE INDEX idx_iam_asset_owner_member ON iam_asset_owner(team_id, member_id);
CREATE TRIGGER trg_iam_asset_owner_tenant BEFORE INSERT ON iam_asset_owner FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE iam_asset_owner ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_iam_asset_owner_rls ON iam_asset_owner
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE iam_asset_perm (
  team_id          BIGINT NOT NULL,
  target_id        BIGINT NOT NULL,
  member_id        BIGINT NOT NULL,
  permission       TEXT NOT NULL CHECK (permission IN ('full','edit','exec','view','none')),
  concurrent_quota BIGINT NOT NULL DEFAULT 0 CHECK (concurrent_quota >= 0),
  sandbox_quota    BIGINT NOT NULL DEFAULT 0 CHECK (sandbox_quota >= 0),
  version          BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
  created_by       BIGINT,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by       BIGINT,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (team_id, target_id, member_id)
);
CREATE INDEX idx_iam_asset_perm_member ON iam_asset_perm(team_id, member_id, target_id);
CREATE TRIGGER trg_iam_asset_perm_tenant BEFORE INSERT ON iam_asset_perm FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_iam_asset_perm_audit BEFORE UPDATE ON iam_asset_perm FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE iam_asset_perm ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_iam_asset_perm_rls ON iam_asset_perm
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

-- GP0-04 迁移基线：多租户 RLS 机制 + 审计字段 + 首个启用 RLS 的表（tnt_team）
-- 约定：所有业务表带 team_id 租户列；写经 gp.set_tenant() 触发器注入 current_setting('gp.team_id')
-- 前缀登记对照 ENGINEERING-SPEC §8

CREATE SCHEMA IF NOT EXISTS gp;

-- 租户注入触发器：写时把 current_setting('gp.team_id') 写入 NEW.team_id
CREATE OR REPLACE FUNCTION gp.set_tenant() RETURNS trigger AS $$
BEGIN
  NEW.team_id := current_setting('gp.team_id', true)::bigint;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

-- 审计字段维护触发器：updated_at/updated_by 自动维护
CREATE OR REPLACE FUNCTION gp.set_audit() RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  NEW.updated_by := NULLIF(current_setting('gp.user_id', true), '')::bigint;
  RETURN NEW;
END $$ LANGUAGE plpgsql;

-- 首个启用 RLS 的基线表：团队（tnt_）
CREATE TABLE tnt_team (
  id          BIGINT PRIMARY KEY,
  team_id     BIGINT NOT NULL,                       -- RLS 租户列
  name        TEXT NOT NULL,
  created_by  BIGINT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by  BIGINT,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at  TIMESTAMPTZ
);

-- 写路径：触发器注入租户 + 审计
CREATE TRIGGER trg_tnt_team_tenant
  BEFORE INSERT ON tnt_team
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();

-- 强制层 RLS
ALTER TABLE tnt_team ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_tnt_team_rls ON tnt_team
  USING (team_id = current_setting('gp.team_id', true)::bigint);

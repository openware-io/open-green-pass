-- GP3-07A：可信域 DB 写边界（只增迁移，不修改 000006/000007）。
--
-- 角色是无密码的 NOLOGIN 组角色；登录角色/Secret 由部署或 DBA bootstrap
-- 提供并授予这些组角色。迁移绝不写入凭据。

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gp_trusted_writer') THEN
    CREATE ROLE gp_trusted_writer NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gp_server') THEN
    CREATE ROLE gp_server NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gp_worker') THEN
    CREATE ROLE gp_worker NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gp_verifier') THEN
    CREATE ROLE gp_verifier NOLOGIN;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gp_report_reader') THEN
    CREATE ROLE gp_report_reader NOLOGIN;
  END IF;
END
$$;

-- 数据库层 append-only：即使调用方绕过应用端口，修改/删除也必须失败。
CREATE OR REPLACE FUNCTION gp.reject_append_only_mutation() RETURNS trigger AS $$
BEGIN
  RAISE EXCEPTION 'gp: % is append-only; % is not permitted', TG_TABLE_NAME, TG_OP
    USING ERRCODE = '55000';
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_aud_event_append_only
  BEFORE UPDATE OR DELETE ON aud_event
  FOR EACH ROW EXECUTE FUNCTION gp.reject_append_only_mutation();

CREATE TRIGGER trg_cost_line_item_append_only
  BEFORE UPDATE OR DELETE ON cost_line_item
  FOR EACH ROW EXECUTE FUNCTION gp.reject_append_only_mutation();

CREATE TRIGGER trg_gate_result_append_only
  BEFORE UPDATE OR DELETE ON gate_result
  FOR EACH ROW EXECUTE FUNCTION gp.reject_append_only_mutation();

-- FORCE RLS 防止表 owner 运行时连接绕过租户策略。
ALTER TABLE aud_event FORCE ROW LEVEL SECURITY;
ALTER TABLE cost_line_item FORCE ROW LEVEL SECURITY;
ALTER TABLE gate_result FORCE ROW LEVEL SECURITY;
ALTER TABLE gate_rule FORCE ROW LEVEL SECURITY;

-- 先收紧默认与运行角色，再按最小权限授予。
REVOKE ALL ON TABLE aud_event, cost_line_item, gate_result, gate_rule
  FROM PUBLIC, gp_server, gp_worker, gp_verifier, gp_report_reader, gp_trusted_writer;

GRANT SELECT ON TABLE aud_event, cost_line_item, gate_result, gate_rule
  TO gp_server, gp_worker, gp_verifier, gp_report_reader;

GRANT SELECT, INSERT ON TABLE aud_event, cost_line_item, gate_result
  TO gp_trusted_writer;
GRANT SELECT, INSERT, UPDATE ON TABLE gate_rule TO gp_trusted_writer;

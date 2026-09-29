-- GP2-05 迁移：服务级截图开关策略（PRD R-TEST-13，防截图证据在高并发下的性能开销）
CREATE TABLE svc_policy (
  id                 BIGINT PRIMARY KEY,
  team_id            BIGINT NOT NULL,
  target_id          BIGINT NOT NULL,
  scenario_id        BIGINT NOT NULL,
  screenshot_enabled BOOLEAN NOT NULL DEFAULT true,  -- 是否需要截图证据；false=证据=日志+哈希
  updated_by         BIGINT,
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, target_id, scenario_id)
);
CREATE INDEX idx_policy_target ON svc_policy(team_id, target_id);

CREATE TRIGGER trg_policy_tenant BEFORE INSERT ON svc_policy FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_policy_audit BEFORE UPDATE ON svc_policy FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE svc_policy ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_svc_policy_rls ON svc_policy
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- GP1-07 迁移：测试报告（rpt_report，工程/服务/用例分级，预留跨场景合编 P2）
CREATE TABLE rpt_report (
  id          BIGINT PRIMARY KEY,
  team_id     BIGINT NOT NULL,
  run_id      BIGINT NOT NULL,
  target_id   BIGINT,
  kind        TEXT NOT NULL,           -- project/service/case（报告粒度）
  title       TEXT,
  version     TEXT,
  branch      TEXT,
  scenario    TEXT,
  status      TEXT NOT NULL,           -- pass/fail/blocked
  summary     JSONB,                   -- 通过率/失败数/成本等汇总
  evidence_ref JSONB,                  -- 证据清单（哈希标注）
  report_html TEXT,                    -- 生成后的 HTML 报告
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by  BIGINT
);
CREATE INDEX idx_rpt_run ON rpt_report(team_id, run_id);
CREATE INDEX idx_rpt_target ON rpt_report(team_id, target_id, created_at DESC);

CREATE TRIGGER trg_rpt_tenant BEFORE INSERT ON rpt_report FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_rpt_audit BEFORE UPDATE ON rpt_report FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE rpt_report ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_rpt_report_rls ON rpt_report
  USING (team_id = current_setting('gp.team_id', true)::bigint);

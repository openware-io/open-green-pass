-- GP1-06 迁移：成本明细（cost_line_item，口径单点；唯一写者=trusted，append-only）
-- 幂等键 UNIQUE(idempotency_key) 防重复计量（同 request_id+biz_point 不双计）
-- 总览=Σ明细；存量执行成本与历史对比可证明（同 case 按 case_id 对账）
-- 前缀登记 ENGINEERING-SPEC §8

CREATE TABLE cost_line_item (
  id            BIGINT PRIMARY KEY,
  request_id    TEXT NOT NULL,                -- AI 网关 requestId（贯穿计量）
  idempotency_key TEXT NOT NULL UNIQUE,       -- 幂等键（request_id + biz_point），防重复落账
  team_id       BIGINT NOT NULL,              -- RLS 租户列
  target_id     BIGINT,
  run_id        BIGINT,
  case_id       BIGINT,
  biz_category  TEXT NOT NULL,                -- 成本大类：generate(用例生成)/execute(用例执行)——PRD 两大口径
  biz_point     TEXT NOT NULL,                -- 细类：case_generate/case_execute/gate_judge/analysis
  model         TEXT,
  tokens_in     BIGINT NOT NULL DEFAULT 0,
  tokens_out    BIGINT NOT NULL DEFAULT 0,
  unit_price    NUMERIC NOT NULL DEFAULT 0,   -- 单价（每千 token）
  amount        NUMERIC NOT NULL DEFAULT 0,   -- 成本金额（元）
  occurred_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_cost_run_case ON cost_line_item(team_id, run_id, case_id, occurred_at DESC);
CREATE INDEX idx_cost_team_cat ON cost_line_item(team_id, biz_category, occurred_at DESC);

CREATE TRIGGER trg_cost_tenant BEFORE INSERT ON cost_line_item FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
-- cost_line_item 不设 set_audit（append-only，无 UPDATE）

ALTER TABLE cost_line_item ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_cost_line_item_rls ON cost_line_item
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- GP1-05 迁移：门禁（gate_rule/gate_result）+ 审计哈希链（aud_event）
-- 策略即代码：Rego 进 git（deploy/rego/），gate_rule 装载策略；判定结果入 aud_event 审计链
-- 审计/门禁只经 trusted 域端口写入（DB 层由 gp_trusted_writer 独占写，对应 ENGINEERING-SPEC §8.1）
-- 前缀登记 ENGINEERING-SPEC §8

-- ========= 审计哈希链（append-only，禁 UPDATE/DELETE）=========
CREATE TABLE aud_event (
  id        BIGINT PRIMARY KEY,
  team_id   BIGINT NOT NULL,               -- RLS 租户列
  actor     BIGINT,                        -- 操作人
  op        TEXT NOT NULL,                 -- gate.evaluate / cost.insert / ...
  asset     TEXT NOT NULL,                 -- 对象类型
  asset_id  BIGINT,
  payload   JSONB NOT NULL DEFAULT '{}',
  prev_hash CHAR(64) NOT NULL DEFAULT repeat('0', 64),
  hash      CHAR(64) NOT NULL,             -- sha256(prev_hash || op || asset_id || payload || ts)
  ts        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_aud_event_team_ts ON aud_event(team_id, asset, asset_id, ts DESC);

CREATE TRIGGER trg_aud_event_tenant BEFORE INSERT ON aud_event FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
-- aud_event 不设 set_audit 触发器（append-only，无 UPDATE）

ALTER TABLE aud_event ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_aud_event_rls ON aud_event
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- ========= 门禁规则（策略即代码，Rego）=========
CREATE TABLE gate_rule (
  id          BIGINT PRIMARY KEY,
  team_id     BIGINT NOT NULL,             -- RLS 租户列
  target_id   BIGINT NOT NULL,
  scenario_id BIGINT NOT NULL,
  rego        TEXT NOT NULL,               -- Rego 策略体
  version     INT NOT NULL DEFAULT 1,
  enabled     BOOL NOT NULL DEFAULT true,
  created_by  BIGINT,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by  BIGINT,
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, target_id, scenario_id, version)
);

CREATE TRIGGER trg_gate_rule_tenant BEFORE INSERT ON gate_rule FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_gate_rule_audit BEFORE UPDATE ON gate_rule FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE gate_rule ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gate_rule_rls ON gate_rule
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- ========= 门禁判定结果 =========
CREATE TABLE gate_result (
  id         BIGINT PRIMARY KEY,
  team_id    BIGINT NOT NULL,             -- RLS 租户列
  run_id     BIGINT NOT NULL,
  rule_id    BIGINT NOT NULL,
  result     TEXT NOT NULL,               -- pass / fail / blocked
  detail     JSONB NOT NULL DEFAULT '{}',
  decided_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_gate_result_run ON gate_result(run_id);

CREATE TRIGGER trg_gate_result_tenant BEFORE INSERT ON gate_result FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_gate_result_audit BEFORE UPDATE ON gate_result FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE gate_result ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gate_result_rls ON gate_result
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- ========= trusted 写者角色（独占审计/门禁写权限）=========
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gp_trusted_writer') THEN
    CREATE ROLE gp_trusted_writer;
  END IF;
END $$;

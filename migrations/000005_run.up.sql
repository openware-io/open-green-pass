-- GP1-04 迁移：测试运行（run_run）+ 用例执行结果（run_case_result）
-- 状态机：queued→version_check→scheduled→running→collect→gate→report→done/failed
-- run_case_result 含 attempt_seq（同 run 内重试/重跑递增）；RLS 与前置迁移一致；前缀登记 ENGINEERING-SPEC §8

-- ========= 测试运行（时间维主键）=========
CREATE TABLE run_run (
  id             BIGINT PRIMARY KEY,
  team_id        BIGINT NOT NULL,               -- RLS 租户列
  scenario_id    BIGINT NOT NULL,               -- 测试场景（api/contract/load/...）
  target_id      BIGINT NOT NULL,               -- 被测对象节点
  env            TEXT NOT NULL DEFAULT 'test',  -- 校验/执行环境
  target_version TEXT NOT NULL,                 -- 目标版本（版本校验前置依据）
  target_branch  TEXT NOT NULL,                 -- 目标分支
  env_version    TEXT,                          -- 环境运行版本（校验结果回填，可空）
  env_check_id   BIGINT,                        -- 关联版本校验记录（可空）
  run_mode       TEXT NOT NULL DEFAULT 'manual',-- manual / ci / trigger
  state          TEXT NOT NULL DEFAULT 'queued',-- queued/version_check/scheduled/running/paused/collect/gate/report/done/failed
  selected_cases JSONB NOT NULL DEFAULT '[]',   -- 当次勾选用例 id 列表；空数组=全量
  started_at     TIMESTAMPTZ,
  ended_at       TIMESTAMPTZ,
  created_by     BIGINT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by     BIGINT,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_run_run_target ON run_run(team_id, target_id, started_at DESC);

CREATE TRIGGER trg_run_run_tenant BEFORE INSERT ON run_run FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_run_run_audit BEFORE UPDATE ON run_run FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE run_run ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_run_run_rls ON run_run
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- ========= 用例执行结果（成本/证据/历史对比的明细粒）=========
CREATE TABLE run_case_result (
  id            BIGINT PRIMARY KEY,
  team_id       BIGINT NOT NULL,               -- RLS 租户列
  run_id        BIGINT NOT NULL,
  case_id       BIGINT NOT NULL,
  case_version  INT NOT NULL,                  -- 执行的用例版本
  status        TEXT NOT NULL,                 -- pass / fail / blocked / skipped / retried
  result_text   TEXT,                          -- 断言/失败/阻断信息
  evidence_ref  JSONB,                         -- {screenshots:[s3uri], logs:[s3uri], hash:sha256}
  ai_tokens_in  BIGINT NOT NULL DEFAULT 0,     -- 本用例执行 AI 输入 token（成本单点计量回填）
  ai_tokens_out BIGINT NOT NULL DEFAULT 0,
  cost_amount   NUMERIC NOT NULL DEFAULT 0,    -- 本用例执行成本（口径单点）
  attempt_seq   INT NOT NULL DEFAULT 1,        -- 同 run 内重试/重跑序号
  started_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  ended_at      TIMESTAMPTZ,
  created_by     BIGINT,
  updated_by     BIGINT,
  UNIQUE (run_id, case_id, attempt_seq)
);
CREATE INDEX idx_run_case_result_run ON run_case_result(run_id, attempt_seq);

CREATE TRIGGER trg_run_case_result_tenant BEFORE INSERT ON run_case_result FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_run_case_result_audit BEFORE UPDATE ON run_case_result FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE run_case_result ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_run_case_result_rls ON run_case_result
  USING (team_id = current_setting('gp.team_id', true)::bigint);

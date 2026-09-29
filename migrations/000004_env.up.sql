-- GP1-03 迁移：测试环境版本校验（"测了没白测"）
-- 测试环境运行版本 env_runtime + 版本校验记录 env_check（目标 vs 环境运行版本）
-- mismatch 阻断执行并入审计链；RLS 模式与 000001/000002/000003 一致
-- 前缀登记对照 ENGINEERING-SPEC §8

-- ========= 测试环境运行版本 =========
CREATE TABLE env_runtime (
  id              BIGINT PRIMARY KEY,
  team_id         BIGINT NOT NULL,               -- RLS 租户列
  target_id       BIGINT NOT NULL,               -- 归属被测对象节点（服务/工程）
  env             TEXT NOT NULL,                 -- 环境标识（test / staging / prod）
  running_version TEXT NOT NULL,                 -- 环境当前运行版本
  checked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by      BIGINT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by      BIGINT,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, target_id, env)
);

CREATE TRIGGER trg_env_runtime_tenant BEFORE INSERT ON env_runtime FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_env_runtime_audit BEFORE UPDATE ON env_runtime FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE env_runtime ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_env_runtime_rls ON env_runtime
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- ========= 版本校验记录（目标 vs 环境运行版本）=========
CREATE TABLE env_check (
  id              BIGINT PRIMARY KEY,
  team_id         BIGINT NOT NULL,               -- RLS 租户列
  run_id          BIGINT,                        -- 关联运行（可空：独立校验）
  target_id       BIGINT NOT NULL,
  target_version  TEXT NOT NULL,                 -- 目标版本（绑定的分支版本）
  env_version     TEXT NOT NULL,                 -- 环境运行版本
  result          TEXT NOT NULL,                 -- match / mismatch / unknown
  checked_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by      BIGINT
);
CREATE INDEX idx_env_check_target ON env_check(team_id, target_id, checked_at DESC);

CREATE TRIGGER trg_env_check_tenant BEFORE INSERT ON env_check FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_env_check_audit  BEFORE UPDATE ON env_check FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE env_check ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_env_check_rls ON env_check
  USING (team_id = current_setting('gp.team_id', true)::bigint);

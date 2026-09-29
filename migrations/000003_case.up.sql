-- GP1-02 迁移：用例版本化（变化可辨）
-- 用例主体 cas_case + 版本表 cas_version（只增不改：新增版本/回退=新增指向旧内容的新版本，非物理删）
-- RLS 模式与 000001/000002 一致：写经 gp.set_tenant() 注入 team_id；强制层 policy 按 gp.team_id 过滤
-- 前缀登记对照 ENGINEERING-SPEC §8

-- ========= 用例（版本化主体）=========
CREATE TABLE cas_case (
  id              BIGINT PRIMARY KEY,
  team_id         BIGINT NOT NULL,                -- RLS 租户列
  target_id       BIGINT NOT NULL,                -- 归属被测对象节点（服务/模块）
  code            TEXT NOT NULL,                  -- 用例编号（按树命名空间稳定，如 im-saas-gw-001）
  title           TEXT NOT NULL,
  kind            TEXT NOT NULL,                  -- 场景族：api/web/ui/contract/perf/mobile/weaknet/ai...
  current_version INT NOT NULL DEFAULT 1,         -- 当前生效版本号
  status          TEXT NOT NULL DEFAULT 'active', -- active / archived / deleted
  created_by      BIGINT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by      BIGINT,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at      TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_cas_team_target_code ON cas_case(team_id, target_id, code);

CREATE TRIGGER trg_cas_case_tenant BEFORE INSERT ON cas_case FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_cas_case_audit  BEFORE UPDATE ON cas_case FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE cas_case ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_cas_case_rls ON cas_case
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- ========= 用例版本（核心：变化可辨 + 来源可溯源）=========
CREATE TABLE cas_version (
  id             BIGINT PRIMARY KEY,
  team_id        BIGINT NOT NULL,                -- RLS 租户列
  case_id        BIGINT NOT NULL,
  version        INT NOT NULL,
  change_type    TEXT NOT NULL,                  -- added / updated / deleted / rollback（随迭代可辨）
  source_repo_id BIGINT,                          -- 来源仓库（可溯源）
  source_branch  TEXT,                            -- 来源分支（可溯源）
  script_json    JSONB,                           -- 脚本/HTTP/规则/压测 + 参数/数据/断言
  approved_by    BIGINT,
  approved_at    TIMESTAMPTZ,
  created_by     BIGINT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (case_id, version)
);

CREATE TRIGGER trg_cas_version_tenant BEFORE INSERT ON cas_version FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_cas_version_audit  BEFORE UPDATE ON cas_version FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE cas_version ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_cas_version_rls ON cas_version
  USING (team_id = current_setting('gp.team_id', true)::bigint);

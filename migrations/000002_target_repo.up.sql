-- GP1-01 迁移：被测对象树 + 仓库绑定（来源溯源）
-- 被测对象树节点 tgt_target（L0 工程/L1 服务组/L2 服务/L3 模块）+ 被测仓库 repo_repo + 分支/版本 repo_branch
-- RLS 模式与 000001 一致：写经 gp.set_tenant() 注入 team_id；强制层 policy 按 gp.team_id 过滤
-- 前缀登记对照 ENGINEERING-SPEC §8

-- ========= 被测对象树 =========
CREATE TABLE tgt_target (
  id            BIGINT PRIMARY KEY,
  team_id       BIGINT NOT NULL,                  -- RLS 租户列
  parent_id     BIGINT,                            -- 顶层为 NULL；层级 parent 链
  level         SMALLINT NOT NULL,                 -- 0 工程 / 1 服务组 / 2 服务 / 3 模块
  name          TEXT NOT NULL,
  kind          TEXT NOT NULL,                     -- api_service / web_app / mobile_app / contract / ai_model / other
  repo_id       BIGINT,                            -- 来源溯源：绑定被测仓库（工程/服务级可绑）
  model_binding JSONB,                             -- 本节点可选 AI 模型绑定（每工程可自选模型）
  status        TEXT NOT NULL DEFAULT 'active',    -- active / archived
  created_by    BIGINT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by    BIGINT,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at    TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_tgt_team_parent_name ON tgt_target(team_id, parent_id, name);

-- 写路径：租户 + 审计注入
CREATE TRIGGER trg_tgt_target_tenant BEFORE INSERT ON tgt_target FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_tgt_target_audit  BEFORE UPDATE ON tgt_target FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

-- 强制层 RLS
ALTER TABLE tgt_target ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_tgt_target_rls ON tgt_target
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- ========= 被测仓库（来源溯源：被测对象树 ← 仓库）=========
CREATE TABLE repo_repo (
  id             BIGINT PRIMARY KEY,
  team_id        BIGINT NOT NULL,                  -- RLS 租户列
  target_id      BIGINT NOT NULL,                  -- 归属被测对象节点（工程/服务）
  kind           TEXT NOT NULL DEFAULT 'git',      -- git / svn / ...
  url            TEXT NOT NULL,                    -- 仓库地址
  default_branch TEXT NOT NULL,                    -- 默认分支
  cred_ref       TEXT,                             -- 凭据引用（密钥不落库，仅引用 Secret 名称）
  created_by     BIGINT,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by     BIGINT,
  updated_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at     TIMESTAMPTZ
);
CREATE UNIQUE INDEX uk_repo_team_target_url ON repo_repo(team_id, target_id, url);

CREATE TRIGGER trg_repo_repo_tenant BEFORE INSERT ON repo_repo FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_repo_repo_audit  BEFORE UPDATE ON repo_repo FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE repo_repo ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_repo_repo_rls ON repo_repo
  USING (team_id = current_setting('gp.team_id', true)::bigint);

-- ========= 仓库分支/版本 =========
CREATE TABLE repo_branch (
  id       BIGINT PRIMARY KEY,
  team_id  BIGINT NOT NULL,                        -- RLS 租户列（经归属 repo 的团队）
  repo_id  BIGINT NOT NULL,
  branch   TEXT NOT NULL,
  version  TEXT NOT NULL,                          -- 运行/目标版本（CICD 可推送）
  head_sha TEXT,                                   -- 分支头 commit
  created_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  deleted_at TIMESTAMPTZ,
  UNIQUE (repo_id, branch)
);

CREATE TRIGGER trg_repo_branch_tenant BEFORE INSERT ON repo_branch FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_repo_branch_audit BEFORE UPDATE ON repo_branch FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE repo_branch ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_repo_branch_rls ON repo_branch
  USING (team_id = current_setting('gp.team_id', true)::bigint);

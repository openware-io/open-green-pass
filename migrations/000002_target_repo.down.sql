-- GP1-01 回滚：删除被测对象树/仓库相关表（先删依赖表的 policy/trigger，再删表）
DROP POLICY IF EXISTS p_repo_branch_rls ON repo_branch;
DROP TRIGGER IF EXISTS trg_repo_branch_tenant ON repo_branch;
DROP TRIGGER IF EXISTS trg_repo_branch_audit  ON repo_branch;
DROP TABLE IF EXISTS repo_branch;

DROP POLICY IF EXISTS p_repo_repo_rls ON repo_repo;
DROP TRIGGER IF EXISTS trg_repo_repo_tenant ON repo_repo;
DROP TRIGGER IF EXISTS trg_repo_repo_audit  ON repo_repo;
DROP TABLE IF EXISTS repo_repo;

DROP POLICY IF EXISTS p_tgt_target_rls ON tgt_target;
DROP TRIGGER IF EXISTS trg_tgt_target_tenant ON tgt_target;
DROP TRIGGER IF EXISTS trg_tgt_target_audit  ON tgt_target;
DROP TABLE IF EXISTS tgt_target;

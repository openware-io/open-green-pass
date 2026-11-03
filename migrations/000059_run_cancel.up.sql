-- 用户主动停止运行（终态 cancelled）所需的审计字段。
-- 迁移只加不改；cancelled_at/cancel_reason 用于执行历史与审计追溯。
ALTER TABLE run_run
  ADD COLUMN IF NOT EXISTS cancelled_at TIMESTAMPTZ,
  ADD COLUMN IF NOT EXISTS cancel_reason TEXT;

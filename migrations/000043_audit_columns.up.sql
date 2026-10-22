-- Fix audit triggers: gp.set_audit() writes updated_by on mutable tables.
ALTER TABLE cov_cell ADD COLUMN updated_by BIGINT;
ALTER TABLE run_fixture ADD COLUMN updated_by BIGINT;
ALTER TABLE ctr_endpoint ADD COLUMN updated_by BIGINT;

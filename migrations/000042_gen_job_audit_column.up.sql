-- Fix gen_job audit trigger: gp.set_audit() writes updated_by.
ALTER TABLE gen_job ADD COLUMN updated_by BIGINT;

DROP TABLE IF EXISTS tgt_service;
ALTER TABLE repo_repo RENAME COLUMN project_id TO target_id;
ALTER TABLE tgt_target ADD COLUMN repo_id BIGINT;


ALTER TABLE env_runtime DROP COLUMN IF EXISTS base_url;
DROP INDEX IF EXISTS uk_gen_review_team_request;
ALTER TABLE gen_seed DROP COLUMN IF EXISTS case_id;
DROP INDEX IF EXISTS uk_gen_snapshot_batch;
ALTER TABLE gen_source_snapshot
  DROP COLUMN IF EXISTS normalized_context,
  DROP COLUMN IF EXISTS source_content,
  DROP COLUMN IF EXISTS source_commit;

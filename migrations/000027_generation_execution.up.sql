-- Durable bounded source snapshots, approved-seed linkage, and executable HTTP target configuration.
ALTER TABLE gen_source_snapshot
  ADD COLUMN source_commit TEXT NOT NULL DEFAULT '',
  ADD COLUMN source_content JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN normalized_context JSONB NOT NULL DEFAULT '{}'::jsonb;
-- Existing pre-027 snapshots were metadata-only and may have multiple rows per
-- batch. Keep them intact; only fully frozen snapshots participate in the
-- one-snapshot-per-batch constraint used by the new worker.
CREATE UNIQUE INDEX uk_gen_snapshot_batch ON gen_source_snapshot(team_id, batch_id)
  WHERE source_commit <> '';

ALTER TABLE gen_seed
  ADD COLUMN case_id BIGINT REFERENCES cas_case(id);

CREATE UNIQUE INDEX uk_gen_review_team_request
  ON gen_review(team_id, request_id);

ALTER TABLE env_runtime
  ADD COLUMN base_url TEXT NOT NULL DEFAULT '';

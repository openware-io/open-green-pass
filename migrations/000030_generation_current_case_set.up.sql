-- A generation is an iteration over one current case set. The append-only
-- cas_version table still preserves audit history but is not a user-visible
-- multi-version case library.
ALTER TABLE gen_seed ADD COLUMN case_key TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_gen_seed_batch_case_key ON gen_seed(team_id, batch_id, case_key);

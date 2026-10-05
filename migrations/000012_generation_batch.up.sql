-- GP3-02A: durable, tenant-scoped projection for the W1 generation pipeline.
-- Large source/prompt/response payloads stay in controlled object storage; the
-- database records only hashes and object references necessary for recovery.

CREATE TABLE gen_batch (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  target_id BIGINT NOT NULL,
  repo_id BIGINT NOT NULL,
  branch TEXT NOT NULL CHECK (btrim(branch) <> ''),
  requested_version TEXT,
  head_sha TEXT,
  state TEXT NOT NULL CHECK (state IN ('enqueued','fetching','parsing','generating','quality_check','review','committing','approved','rejected','rolled_back','failed')),
  state_version BIGINT NOT NULL DEFAULT 1 CHECK (state_version > 0),
  workflow_id TEXT,
  idempotency_key TEXT NOT NULL CHECK (btrim(idempotency_key) <> ''),
  parent_batch_id BIGINT REFERENCES gen_batch(id),
  failure_summary TEXT,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, idempotency_key)
);
CREATE INDEX idx_gen_batch_target_created ON gen_batch(team_id, target_id, created_at DESC);
CREATE INDEX idx_gen_batch_workflow ON gen_batch(team_id, workflow_id) WHERE workflow_id IS NOT NULL;
CREATE TRIGGER trg_gen_batch_tenant BEFORE INSERT ON gen_batch FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_gen_batch_audit BEFORE UPDATE ON gen_batch FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE gen_batch ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_batch_rls ON gen_batch
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE gen_source_snapshot (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  batch_id BIGINT NOT NULL REFERENCES gen_batch(id),
  selector TEXT NOT NULL,
  content_hash TEXT NOT NULL,
  parser_version TEXT NOT NULL,
  object_uri TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (batch_id, selector, content_hash)
);
CREATE TRIGGER trg_gen_source_snapshot_tenant BEFORE INSERT ON gen_source_snapshot FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE gen_source_snapshot ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_source_snapshot_rls ON gen_source_snapshot
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE gen_seed (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  batch_id BIGINT NOT NULL REFERENCES gen_batch(id),
  source_snapshot_id BIGINT REFERENCES gen_source_snapshot(id),
  source_locator TEXT NOT NULL,
  normalized_spec JSONB NOT NULL,
  dedupe_hash TEXT NOT NULL,
  quality_state TEXT NOT NULL DEFAULT 'pending' CHECK (quality_state IN ('pending','passed','failed')),
  approval_state TEXT NOT NULL DEFAULT 'pending' CHECK (approval_state IN ('pending','approved','rejected')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (batch_id, dedupe_hash)
);
CREATE INDEX idx_gen_seed_batch ON gen_seed(team_id, batch_id, id);
CREATE TRIGGER trg_gen_seed_tenant BEFORE INSERT ON gen_seed FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE gen_seed ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_seed_rls ON gen_seed
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE gen_review (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  batch_id BIGINT NOT NULL REFERENCES gen_batch(id),
  actor_id BIGINT NOT NULL,
  decision TEXT NOT NULL CHECK (decision IN ('approve','reject','rollback')),
  comment TEXT,
  state_version BIGINT NOT NULL CHECK (state_version > 0),
  request_id TEXT NOT NULL,
  occurred_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_gen_review_batch ON gen_review(team_id, batch_id, occurred_at DESC);
CREATE TRIGGER trg_gen_review_tenant BEFORE INSERT ON gen_review FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE gen_review ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_review_rls ON gen_review
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE gen_step (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  batch_id BIGINT NOT NULL REFERENCES gen_batch(id),
  activity_name TEXT NOT NULL,
  attempt INTEGER NOT NULL DEFAULT 1 CHECK (attempt > 0),
  status TEXT NOT NULL CHECK (status IN ('pending','running','succeeded','failed')),
  error_summary TEXT,
  started_at TIMESTAMPTZ,
  ended_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (batch_id, activity_name, attempt)
);
CREATE INDEX idx_gen_step_batch ON gen_step(team_id, batch_id, id);
CREATE TRIGGER trg_gen_step_tenant BEFORE INSERT ON gen_step FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE gen_step ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_step_rls ON gen_step
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

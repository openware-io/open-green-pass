-- M1: DSH generation job ledger (AI domain, gen_ prefix).
CREATE TABLE gen_job (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  project_id BIGINT NOT NULL,
  service_id BIGINT,
  module TEXT NOT NULL DEFAULT '',
  state TEXT NOT NULL DEFAULT 'queued'
    CHECK (state IN ('queued','running','succeeded','failed','timeout','cancelled')),
  external_job_id TEXT NOT NULL DEFAULT '',
  prompt TEXT NOT NULL,
  timeout_ms BIGINT NOT NULL DEFAULT 900000,
  output TEXT NOT NULL DEFAULT '',
  error TEXT NOT NULL DEFAULT '',
  idempotency_key TEXT NOT NULL,
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  started_at TIMESTAMPTZ,
  finished_at TIMESTAMPTZ,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, idempotency_key)
);
CREATE INDEX idx_gen_job_team_state ON gen_job(team_id, state, created_at DESC);
CREATE INDEX idx_gen_job_external ON gen_job(team_id, external_job_id) WHERE external_job_id <> '';

CREATE TRIGGER trg_gen_job_tenant BEFORE INSERT ON gen_job
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_gen_job_audit BEFORE UPDATE ON gen_job
  FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE gen_job ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_job_rls ON gen_job
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE gen_job_event (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  job_id BIGINT NOT NULL REFERENCES gen_job(id) ON DELETE CASCADE,
  seq INT NOT NULL,
  type TEXT NOT NULL,
  state TEXT NOT NULL DEFAULT '',
  message TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (job_id, seq)
);
CREATE INDEX idx_gen_job_event_job ON gen_job_event(team_id, job_id, seq);

CREATE TRIGGER trg_gen_job_event_tenant BEFORE INSERT ON gen_job_event
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE gen_job_event ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_job_event_rls ON gen_job_event
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

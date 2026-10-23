-- GenerationRun groups all service-level generation units created by one
-- operator action. gen_batch remains the durable execution unit so the existing
-- DSH/Temporal pipeline can be reused without duplicating its state machine.
CREATE TABLE gen_run (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  project_id BIGINT NOT NULL,
  scope_type TEXT NOT NULL CHECK (scope_type IN ('project','repo','service')),
  scope_id BIGINT NOT NULL,
  state TEXT NOT NULL CHECK (state IN ('queued','running','succeeded','partial_failed','failed','cancelled')),
  total_units INT NOT NULL CHECK (total_units > 0),
  completed_units INT NOT NULL DEFAULT 0,
  failed_units INT NOT NULL DEFAULT 0,
  tokens_in BIGINT NOT NULL DEFAULT 0,
  tokens_out BIGINT NOT NULL DEFAULT 0,
  cost_amount NUMERIC(18,6) NOT NULL DEFAULT 0,
  idempotency_key TEXT NOT NULL CHECK (btrim(idempotency_key) <> ''),
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, idempotency_key)
);
CREATE INDEX idx_gen_run_project_created ON gen_run(team_id, project_id, created_at DESC);
CREATE TRIGGER trg_gen_run_tenant BEFORE INSERT ON gen_run FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_gen_run_audit BEFORE UPDATE ON gen_run FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE gen_run ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_run_rls ON gen_run
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE gen_run_unit (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  run_id BIGINT NOT NULL REFERENCES gen_run(id),
  service_id BIGINT NOT NULL,
  repo_id BIGINT NOT NULL,
  batch_id BIGINT REFERENCES gen_batch(id),
  state TEXT NOT NULL CHECK (state IN ('queued','running','succeeded','failed','cancelled')),
  error_summary TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (run_id, service_id)
);
CREATE INDEX idx_gen_run_unit_run ON gen_run_unit(team_id, run_id, id);
CREATE TRIGGER trg_gen_run_unit_tenant BEFORE INSERT ON gen_run_unit FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_gen_run_unit_audit BEFORE UPDATE ON gen_run_unit FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE gen_run_unit ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_run_unit_rls ON gen_run_unit
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

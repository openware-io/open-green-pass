-- M0/M1: coverage ledger (Governance Coverage, cov_ prefix). Greenfield.
CREATE TABLE cov_cell (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  project_id BIGINT NOT NULL,
  service_id BIGINT,
  endpoint_id BIGINT,
  endpoint_hash TEXT NOT NULL,
  method TEXT NOT NULL,
  path TEXT NOT NULL,
  dimension TEXT NOT NULL,
  equivalence_class TEXT NOT NULL,
  role TEXT NOT NULL DEFAULT 'any',
  state TEXT NOT NULL DEFAULT 'none',
  required BOOLEAN NOT NULL DEFAULT true,
  priority TEXT NOT NULL DEFAULT 'P1' CHECK (priority IN ('P0','P1','P2','P3')),
  status TEXT NOT NULL DEFAULT 'planned'
    CHECK (status IN ('planned','generated','executed','passed','failed','blocked','waived')),
  case_id BIGINT,
  waiver_id BIGINT,
  evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, endpoint_hash, dimension, equivalence_class, role, state)
);
CREATE INDEX idx_cov_cell_team_status ON cov_cell(team_id, project_id, status, priority);
CREATE INDEX idx_cov_cell_case ON cov_cell(team_id, case_id) WHERE case_id IS NOT NULL;

CREATE TRIGGER trg_cov_cell_tenant BEFORE INSERT ON cov_cell
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_cov_cell_audit BEFORE UPDATE ON cov_cell
  FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE cov_cell ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_cov_cell_rls ON cov_cell
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE cov_waiver (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  cell_id BIGINT NOT NULL REFERENCES cov_cell(id) ON DELETE CASCADE,
  reason TEXT NOT NULL,
  owner_id BIGINT NOT NULL,
  approved_by BIGINT,
  expires_at TIMESTAMPTZ,
  evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_cov_waiver_cell ON cov_waiver(team_id, cell_id, expires_at);

CREATE TRIGGER trg_cov_waiver_tenant BEFORE INSERT ON cov_waiver
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE cov_waiver ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_cov_waiver_rls ON cov_waiver
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

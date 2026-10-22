-- M2: per-run fixture namespace and environment lease (Execution, run_/env_ prefixes).
CREATE TABLE run_fixture (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  run_id BIGINT NOT NULL,
  namespace TEXT NOT NULL,
  manifest JSONB NOT NULL DEFAULT '{}'::jsonb,
  state TEXT NOT NULL DEFAULT 'preparing'
    CHECK (state IN ('preparing','ready','cleaning','cleaned','failed')),
  expires_at TIMESTAMPTZ,
  cleanup_state TEXT NOT NULL DEFAULT 'pending'
    CHECK (cleanup_state IN ('pending','running','done','failed')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, run_id, namespace)
);
CREATE INDEX idx_run_fixture_team_state ON run_fixture(team_id, run_id, state);
CREATE INDEX idx_run_fixture_expiry ON run_fixture(team_id, expires_at) WHERE expires_at IS NOT NULL;

CREATE TRIGGER trg_run_fixture_tenant BEFORE INSERT ON run_fixture
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_run_fixture_audit BEFORE UPDATE ON run_fixture
  FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE run_fixture ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_run_fixture_rls ON run_fixture
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE env_lease (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  env_id BIGINT NOT NULL,
  run_id BIGINT NOT NULL,
  owner TEXT NOT NULL,
  lease_until TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_env_lease_env ON env_lease(team_id, env_id, lease_until);

CREATE TRIGGER trg_env_lease_tenant BEFORE INSERT ON env_lease
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE env_lease ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_env_lease_rls ON env_lease
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

DROP POLICY IF EXISTS p_env_lease_rls ON env_lease;
DROP TRIGGER IF EXISTS trg_env_lease_tenant ON env_lease;
DROP TABLE IF EXISTS env_lease;

DROP POLICY IF EXISTS p_run_fixture_rls ON run_fixture;
DROP TRIGGER IF EXISTS trg_run_fixture_audit ON run_fixture;
DROP TRIGGER IF EXISTS trg_run_fixture_tenant ON run_fixture;
DROP TABLE IF EXISTS run_fixture;

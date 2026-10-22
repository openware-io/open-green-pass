-- M1: contract inventory used by the coverage planner (Governance Contract, ctr_ prefix).
CREATE TABLE ctr_endpoint (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  project_id BIGINT NOT NULL,
  service_id BIGINT,
  module TEXT NOT NULL DEFAULT '',
  method TEXT NOT NULL CHECK (method IN ('GET','POST','PUT','PATCH','DELETE','HEAD','OPTIONS')),
  path TEXT NOT NULL,
  operation_id TEXT NOT NULL DEFAULT '',
  auth_mode TEXT NOT NULL DEFAULT 'bearer'
    CHECK (auth_mode IN ('bearer','public','presigned','internal')),
  source_file TEXT NOT NULL DEFAULT '',
  source_line INT,
  request_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  response_schema JSONB NOT NULL DEFAULT '{}'::jsonb,
  version INT NOT NULL DEFAULT 1,
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, project_id, method, path, version)
);
CREATE INDEX idx_ctr_endpoint_team_module ON ctr_endpoint(team_id, project_id, module, path);

CREATE TRIGGER trg_ctr_endpoint_tenant BEFORE INSERT ON ctr_endpoint
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_ctr_endpoint_audit BEFORE UPDATE ON ctr_endpoint
  FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE ctr_endpoint ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_ctr_endpoint_rls ON ctr_endpoint
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE ctr_parameter (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  endpoint_id BIGINT NOT NULL REFERENCES ctr_endpoint(id) ON DELETE CASCADE,
  location TEXT NOT NULL CHECK (location IN ('path','query','header','body')),
  name TEXT NOT NULL,
  type TEXT NOT NULL DEFAULT 'string',
  required BOOLEAN NOT NULL DEFAULT false,
  enum_values JSONB NOT NULL DEFAULT '[]'::jsonb,
  min_value NUMERIC,
  max_value NUMERIC,
  pattern TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_ctr_parameter_endpoint ON ctr_parameter(team_id, endpoint_id);
CREATE TRIGGER trg_ctr_parameter_tenant BEFORE INSERT ON ctr_parameter
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE ctr_parameter ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_ctr_parameter_rls ON ctr_parameter
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE ctr_error (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  endpoint_id BIGINT NOT NULL REFERENCES ctr_endpoint(id) ON DELETE CASCADE,
  http_status INT NOT NULL,
  code TEXT NOT NULL,
  message TEXT NOT NULL DEFAULT '',
  source TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_ctr_error_endpoint ON ctr_error(team_id, endpoint_id, http_status);
CREATE TRIGGER trg_ctr_error_tenant BEFORE INSERT ON ctr_error
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE ctr_error ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_ctr_error_rls ON ctr_error
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

-- M0-Dev: defect/finding core (Governance Defect, def_ prefix).
-- Greenfield table; no backfill. RLS is mandatory.
CREATE TABLE def_finding (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  project_id BIGINT NOT NULL,
  service_id BIGINT,
  endpoint_id BIGINT,
  target_id BIGINT,
  repository_id BIGINT,
  repo_name TEXT NOT NULL DEFAULT '',
  repo_url TEXT NOT NULL,
  branch TEXT NOT NULL DEFAULT '',
  commit_sha TEXT NOT NULL,
  module TEXT NOT NULL DEFAULT '',
  endpoint TEXT NOT NULL DEFAULT '',
  title TEXT NOT NULL CHECK (btrim(title) <> ''),
  description TEXT NOT NULL DEFAULT '',
  severity TEXT NOT NULL CHECK (severity IN ('P0','P1','P2','P3')),
  category TEXT NOT NULL DEFAULT 'product'
    CHECK (category IN ('product','case','environment','data','expected_change','flaky')),
  status TEXT NOT NULL DEFAULT 'candidate'
    CHECK (status IN ('candidate','triaged','confirmed','fixing','pending_regression','closed','false_positive')),
  expected TEXT NOT NULL DEFAULT '',
  actual TEXT NOT NULL DEFAULT '',
  root_cause TEXT NOT NULL DEFAULT '',
  impact TEXT NOT NULL DEFAULT '',
  confidence TEXT NOT NULL DEFAULT 'medium'
    CHECK (confidence IN ('high','medium','low')),
  reproduction JSONB NOT NULL DEFAULT '{}'::jsonb,
  source_ref JSONB NOT NULL DEFAULT '{}'::jsonb,
  evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
  coverage_cells JSONB NOT NULL DEFAULT '[]'::jsonb,
  owner_id BIGINT,
  linked_run_id BIGINT,
  linked_case_id BIGINT,
  dedupe_hash TEXT NOT NULL,
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, dedupe_hash)
);

CREATE INDEX idx_def_finding_team_status ON def_finding(team_id, status, severity, updated_at DESC);
CREATE INDEX idx_def_finding_team_repo ON def_finding(team_id, repo_url, commit_sha);
CREATE INDEX idx_def_finding_team_run ON def_finding(team_id, linked_run_id)
  WHERE linked_run_id IS NOT NULL;

CREATE TRIGGER trg_def_finding_tenant BEFORE INSERT ON def_finding
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_def_finding_audit BEFORE UPDATE ON def_finding
  FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE def_finding ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_def_finding_rls ON def_finding
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE def_event (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  finding_id BIGINT NOT NULL REFERENCES def_finding(id) ON DELETE CASCADE,
  action TEXT NOT NULL,
  from_status TEXT,
  to_status TEXT,
  actor_id BIGINT NOT NULL DEFAULT 0,
  reason TEXT NOT NULL DEFAULT '',
  payload JSONB NOT NULL DEFAULT '{}'::jsonb,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_def_event_finding ON def_event(team_id, finding_id, created_at);

CREATE TRIGGER trg_def_event_tenant BEFORE INSERT ON def_event
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE def_event ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_def_event_rls ON def_event
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE def_suggestion (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  finding_id BIGINT NOT NULL REFERENCES def_finding(id) ON DELETE CASCADE,
  summary TEXT NOT NULL DEFAULT '',
  approach TEXT NOT NULL DEFAULT '',
  affected_files JSONB NOT NULL DEFAULT '[]'::jsonb,
  confidence TEXT NOT NULL DEFAULT 'medium',
  source TEXT NOT NULL DEFAULT 'human',
  status TEXT NOT NULL DEFAULT 'draft'
    CHECK (status IN ('draft','accepted','rejected')),
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_def_suggestion_finding ON def_suggestion(team_id, finding_id, created_at);

CREATE TRIGGER trg_def_suggestion_tenant BEFORE INSERT ON def_suggestion
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE def_suggestion ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_def_suggestion_rls ON def_suggestion
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

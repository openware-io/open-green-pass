-- Development-stage replacement for the single target.repo_id model.
ALTER TABLE tgt_target DROP COLUMN IF EXISTS repo_id;
ALTER TABLE repo_repo RENAME COLUMN target_id TO project_id;

CREATE TABLE tgt_service (
  id            BIGINT PRIMARY KEY,
  team_id       BIGINT NOT NULL,
  project_id    BIGINT NOT NULL,
  repo_id       BIGINT NOT NULL,
  name          TEXT NOT NULL CHECK (btrim(name) <> ''),
  source_path   TEXT NOT NULL DEFAULT '.' CHECK (btrim(source_path) <> ''),
  kind          TEXT NOT NULL DEFAULT 'other',
  build_context TEXT NOT NULL DEFAULT '.',
  dockerfile_path TEXT,
  manifest_path TEXT,
  status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','disabled')),
  created_by    BIGINT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by    BIGINT,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (repo_id, source_path)
);
CREATE INDEX idx_tgt_service_project ON tgt_service(team_id, project_id, repo_id);
CREATE TRIGGER trg_tgt_service_tenant BEFORE INSERT ON tgt_service FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_tgt_service_audit BEFORE UPDATE ON tgt_service FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE tgt_service ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_tgt_service_rls ON tgt_service
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);


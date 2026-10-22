-- M0/M2: test evidence metadata (Trusted, evd_ prefix). Greenfield.
-- Binary objects live in SeaweedFS/S3; this table stores metadata only.
CREATE TABLE evd_evidence (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  finding_id BIGINT NOT NULL,
  kind TEXT NOT NULL CHECK (kind IN ('request','response','log','trace','screenshot','video')),
  storage_uri TEXT NOT NULL CHECK (btrim(storage_uri) <> ''),
  mime_type TEXT NOT NULL CHECK (btrim(mime_type) <> ''),
  size_bytes BIGINT NOT NULL CHECK (size_bytes > 0),
  width INT,
  height INT,
  duration_ms INT,
  sha256 TEXT NOT NULL CHECK (btrim(sha256) <> ''),
  thumbnail_uri TEXT,
  redacted BOOLEAN NOT NULL DEFAULT false,
  retention_class TEXT NOT NULL DEFAULT 'standard',
  created_by BIGINT NOT NULL DEFAULT 0,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_evd_evidence_finding ON evd_evidence(team_id, finding_id, created_at);
CREATE INDEX idx_evd_evidence_sha ON evd_evidence(team_id, sha256);

CREATE TRIGGER trg_evd_evidence_tenant BEFORE INSERT ON evd_evidence
  FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE evd_evidence ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_evd_evidence_rls ON evd_evidence
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);
ALTER TABLE evd_evidence FORCE ROW LEVEL SECURITY;

CREATE TRIGGER trg_evd_evidence_append_only
  BEFORE UPDATE OR DELETE ON evd_evidence
  FOR EACH ROW EXECUTE FUNCTION gp.reject_append_only_mutation();

-- Trusted write boundary: server/worker can read, only trusted writer can write.
REVOKE ALL ON TABLE evd_evidence FROM PUBLIC, gp_server, gp_worker, gp_trusted_writer;
GRANT SELECT ON TABLE evd_evidence TO gp_server, gp_worker;
GRANT SELECT, INSERT ON TABLE evd_evidence TO gp_trusted_writer;

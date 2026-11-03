-- Append-only generation run event log (AI domain). This is the SSE fact source
-- and the replay cursor (`seq`, monotonic per table). Events are change signals,
-- not snapshots: the run detail remains the authoritative read.
CREATE TABLE gen_run_event (
  id          BIGINT PRIMARY KEY,
  team_id     BIGINT NOT NULL,
  run_id      BIGINT NOT NULL REFERENCES gen_run(id),
  seq         BIGINT GENERATED ALWAYS AS IDENTITY,
  type        TEXT NOT NULL CHECK (type IN (
    'run.created','run.started','run.progress','run.waiting_review',
    'run.completed','run.partial_failed','run.failed','run.cancelled','run.reconciled',
    'unit.queued','unit.stage_changed','unit.review_required',
    'unit.approved','unit.rejected','unit.failed','unit.cancelled')),
  unit_id     BIGINT,
  event_key   TEXT,
  payload     JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(payload::text) <= 4096),
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (run_id, seq)
);
CREATE UNIQUE INDEX uk_gen_run_event_key ON gen_run_event(run_id, event_key) WHERE event_key IS NOT NULL;
CREATE INDEX idx_gen_run_event_seq ON gen_run_event(team_id, run_id, seq);
CREATE INDEX idx_gen_run_event_created ON gen_run_event(created_at);
CREATE TRIGGER trg_gen_run_event_tenant BEFORE INSERT ON gen_run_event FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE gen_run_event ENABLE ROW LEVEL SECURITY;
ALTER TABLE gen_run_event FORCE ROW LEVEL SECURITY;
CREATE POLICY p_gen_run_event_rls ON gen_run_event
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);
-- Even a table owner cannot rewrite history through a direct UPDATE/DELETE.
CREATE TRIGGER trg_gen_run_event_append_only BEFORE UPDATE OR DELETE ON gen_run_event
  FOR EACH ROW EXECUTE FUNCTION gp.reject_append_only_mutation();

-- Least privilege: runtime roles append and read, read-only roles only read.
REVOKE ALL ON TABLE gen_run_event FROM PUBLIC, gp_server, gp_worker, gp_verifier, gp_report_reader, gp_trusted_writer;
GRANT SELECT, INSERT ON TABLE gen_run_event TO gp_server, gp_worker;
GRANT SELECT ON TABLE gen_run_event TO gp_verifier, gp_report_reader;
-- Identity sequences are separate privileges; without USAGE the runtime role
-- cannot append even though it has INSERT on the table.
GRANT USAGE, SELECT ON SEQUENCE gen_run_event_seq_seq TO gp_server, gp_worker;

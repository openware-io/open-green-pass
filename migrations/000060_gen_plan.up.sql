-- Generation preflight plans (AI domain, gen_ prefix).
-- A plan is an immutable snapshot of the resolved units, estimate and risks at
-- confirm time. It is single-use: the run create path re-validates hash/TTL and
-- marks the row consumed. This closes the TOCTOU gap between preview and create
-- without making the browser authoritative.
CREATE TABLE gen_plan (
  id               BIGINT PRIMARY KEY,
  team_id          BIGINT NOT NULL,
  project_id       BIGINT NOT NULL,
  scope_type       TEXT NOT NULL CHECK (scope_type IN ('project','repo','service')),
  scope_id         BIGINT NOT NULL DEFAULT 0,
  plan_hash        TEXT NOT NULL CHECK (btrim(plan_hash) <> ''),
  state            TEXT NOT NULL DEFAULT 'draft' CHECK (state IN ('draft','consumed','expired')),
  request          JSONB NOT NULL DEFAULT '{}'::jsonb,
  summary          JSONB NOT NULL DEFAULT '{}'::jsonb,
  items            JSONB NOT NULL DEFAULT '[]'::jsonb,
  estimate         JSONB NOT NULL DEFAULT '{}'::jsonb,
  risks            JSONB NOT NULL DEFAULT '[]'::jsonb,
  expires_at       TIMESTAMPTZ NOT NULL,
  confirmed_run_id BIGINT,
  created_by       BIGINT NOT NULL,
  created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by       BIGINT,
  updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_gen_plan_project_created ON gen_plan(team_id, project_id, created_at DESC);
-- Idempotent re-plan lookup: only confirmable drafts participate.
CREATE INDEX idx_gen_plan_hash ON gen_plan(team_id, plan_hash) WHERE state = 'draft';
CREATE INDEX idx_gen_plan_expiry ON gen_plan(team_id, expires_at) WHERE state = 'draft';
CREATE TRIGGER trg_gen_plan_tenant BEFORE INSERT ON gen_plan FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_gen_plan_audit BEFORE UPDATE ON gen_plan FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE gen_plan ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_gen_plan_rls ON gen_plan
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

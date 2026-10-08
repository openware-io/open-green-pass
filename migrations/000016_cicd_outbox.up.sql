CREATE TABLE cicd_outbox (
  id bigserial PRIMARY KEY,
  team_id bigint NOT NULL,
  event_id text NOT NULL,
  delivery_key text NOT NULL,
  provider text NOT NULL,
  attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  status text NOT NULL CHECK (status IN ('pending','dispatching','delivered','failed')),
  last_error text NOT NULL DEFAULT '',
  claimed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (team_id, delivery_key)
);
CREATE INDEX idx_cicd_outbox_due ON cicd_outbox(status, next_attempt_at, id);
CREATE INDEX idx_cicd_outbox_team ON cicd_outbox(team_id, created_at DESC);
ALTER TABLE cicd_outbox ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_cicd_outbox_rls ON cicd_outbox
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

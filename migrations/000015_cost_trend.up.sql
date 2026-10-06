-- Rebuildable ordinary PostgreSQL projection. This is not a Timescale hypertable.
CREATE TABLE ts_cost_fact (
  source_cost_id bigint PRIMARY KEY REFERENCES cost_line_item(id) ON DELETE CASCADE,
  team_id bigint NOT NULL,
  target_id bigint NOT NULL,
  run_id bigint NOT NULL,
  case_id bigint NOT NULL,
  biz_category text NOT NULL,
  biz_point text NOT NULL,
  model text NOT NULL,
  tokens_in bigint NOT NULL CHECK (tokens_in >= 0),
  tokens_out bigint NOT NULL CHECK (tokens_out >= 0),
  amount numeric(18,6) NOT NULL CHECK (amount >= 0),
  occurred_at timestamptz NOT NULL
);

CREATE INDEX idx_ts_cost_fact_team_time ON ts_cost_fact(team_id, occurred_at DESC);
CREATE INDEX idx_ts_cost_fact_team_case_time ON ts_cost_fact(team_id, case_id, occurred_at DESC);

ALTER TABLE ts_cost_fact ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_ts_cost_fact_rls ON ts_cost_fact
  USING (team_id = current_setting('app.tenant_id', true)::bigint)
  WITH CHECK (team_id = current_setting('app.tenant_id', true)::bigint);


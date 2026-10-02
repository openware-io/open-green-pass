CREATE TABLE mdl_model (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  name TEXT NOT NULL,
  provider TEXT NOT NULL,
  model_key TEXT NOT NULL,
  capabilities JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL CHECK (status IN ('pending','approved','disabled')),
  secret_ref TEXT,
  endpoint_ref TEXT,
  default_timeout_ms INTEGER NOT NULL DEFAULT 30000,
  max_tokens_in INTEGER NOT NULL DEFAULT 0,
  max_tokens_out INTEGER NOT NULL DEFAULT 0,
  revision INTEGER NOT NULL DEFAULT 1,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE(team_id, provider, model_key)
);
ALTER TABLE mdl_model ENABLE ROW LEVEL SECURITY;
CREATE POLICY mdl_model_team_policy ON mdl_model USING (team_id = current_setting('app.team_id', true)::bigint);

CREATE TABLE cost_price (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  model_id BIGINT NOT NULL REFERENCES mdl_model(id),
  currency TEXT NOT NULL,
  input_per_1k NUMERIC(20,10) NOT NULL CHECK (input_per_1k >= 0),
  output_per_1k NUMERIC(20,10) NOT NULL CHECK (output_per_1k >= 0),
  effective_from TIMESTAMPTZ NOT NULL,
  effective_to TIMESTAMPTZ,
  status TEXT NOT NULL DEFAULT 'active',
  approved_by BIGINT,
  revision INTEGER NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK (effective_to IS NULL OR effective_to > effective_from)
);
ALTER TABLE cost_price ENABLE ROW LEVEL SECURITY;
CREATE POLICY cost_price_team_policy ON cost_price USING (team_id = current_setting('app.team_id', true)::bigint);
CREATE INDEX cost_price_resolve_idx ON cost_price(team_id, model_id, effective_from DESC);

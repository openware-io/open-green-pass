-- Model-aware token pricing. Prices are CNY per 1K tokens and effective from
-- an explicit timestamp so historical cost lines keep their original rate.
CREATE TABLE cost_price (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  model TEXT NOT NULL CHECK (btrim(model) <> ''),
  input_unit_price NUMERIC(18,6) NOT NULL CHECK (input_unit_price >= 0),
  output_unit_price NUMERIC(18,6) NOT NULL CHECK (output_unit_price >= 0),
  cache_read_unit_price NUMERIC(18,6) NOT NULL DEFAULT 0 CHECK (cache_read_unit_price >= 0),
  currency TEXT NOT NULL DEFAULT 'CNY' CHECK (currency = 'CNY'),
  effective_from TIMESTAMPTZ NOT NULL,
  created_by BIGINT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (team_id, model, effective_from)
);
CREATE INDEX idx_cost_price_lookup ON cost_price(team_id, model, effective_from DESC);
CREATE TRIGGER trg_cost_price_tenant BEFORE INSERT ON cost_price FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_cost_price_audit BEFORE UPDATE ON cost_price FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE cost_price ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_cost_price_rls ON cost_price
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

ALTER TABLE cost_line_item
  ADD COLUMN IF NOT EXISTS cache_read_tokens BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS input_unit_price NUMERIC(18,6) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS output_unit_price NUMERIC(18,6) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cache_read_unit_price NUMERIC(18,6) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS price_id BIGINT REFERENCES cost_price(id),
  ADD COLUMN IF NOT EXISTS price_missing BOOLEAN NOT NULL DEFAULT FALSE;

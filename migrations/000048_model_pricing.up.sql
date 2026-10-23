-- Evolve the existing governance cost_price table (000010) into an
-- effective-dated, model-key based tariff. model_id stays for legacy model
-- catalog references but is no longer required for DSH runtime models.
ALTER TABLE cost_price ALTER COLUMN model_id DROP NOT NULL;
ALTER TABLE cost_price ADD COLUMN IF NOT EXISTS model_key TEXT;
ALTER TABLE cost_price ADD COLUMN IF NOT EXISTS cache_read_per_1k NUMERIC(20,10) NOT NULL DEFAULT 0 CHECK (cache_read_per_1k >= 0);
ALTER TABLE cost_price ADD COLUMN IF NOT EXISTS updated_by BIGINT;
ALTER TABLE cost_price ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

DROP POLICY IF EXISTS cost_price_team_policy ON cost_price;
CREATE POLICY cost_price_team_policy ON cost_price
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE UNIQUE INDEX IF NOT EXISTS uk_cost_price_model_effective
  ON cost_price(team_id, model_key, effective_from)
  WHERE model_key IS NOT NULL;

DROP TRIGGER IF EXISTS trg_cost_price_tenant ON cost_price;
CREATE TRIGGER trg_cost_price_tenant BEFORE INSERT ON cost_price FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
DROP TRIGGER IF EXISTS trg_cost_price_audit ON cost_price;
CREATE TRIGGER trg_cost_price_audit BEFORE UPDATE ON cost_price FOR EACH ROW EXECUTE FUNCTION gp.set_audit();

ALTER TABLE cost_line_item
  ADD COLUMN IF NOT EXISTS cache_read_tokens BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS input_unit_price NUMERIC(18,6) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS output_unit_price NUMERIC(18,6) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS cache_read_unit_price NUMERIC(18,6) NOT NULL DEFAULT 0,
  ADD COLUMN IF NOT EXISTS price_id BIGINT REFERENCES cost_price(id),
  ADD COLUMN IF NOT EXISTS price_missing BOOLEAN NOT NULL DEFAULT FALSE;

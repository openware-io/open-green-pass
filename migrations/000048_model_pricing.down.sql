ALTER TABLE cost_line_item
  DROP COLUMN IF EXISTS price_missing,
  DROP COLUMN IF EXISTS price_id,
  DROP COLUMN IF EXISTS cache_read_unit_price,
  DROP COLUMN IF EXISTS output_unit_price,
  DROP COLUMN IF EXISTS input_unit_price,
  DROP COLUMN IF EXISTS cache_read_tokens;

DROP TRIGGER IF EXISTS trg_cost_price_tenant ON cost_price;
DROP TRIGGER IF EXISTS trg_cost_price_audit ON cost_price;
DROP INDEX IF EXISTS uk_cost_price_model_effective;
ALTER TABLE cost_price DROP COLUMN IF EXISTS updated_by;
ALTER TABLE cost_price DROP COLUMN IF EXISTS updated_at;
ALTER TABLE cost_price DROP COLUMN IF EXISTS cache_read_per_1k;
ALTER TABLE cost_price DROP COLUMN IF EXISTS model_key;
ALTER TABLE cost_price ALTER COLUMN model_id SET NOT NULL;
DROP POLICY IF EXISTS cost_price_team_policy ON cost_price;
CREATE POLICY cost_price_team_policy ON cost_price
  USING (team_id = current_setting('app.team_id', true)::bigint);

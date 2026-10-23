ALTER TABLE cost_line_item
  DROP COLUMN IF EXISTS price_missing,
  DROP COLUMN IF EXISTS price_id,
  DROP COLUMN IF EXISTS cache_read_unit_price,
  DROP COLUMN IF EXISTS output_unit_price,
  DROP COLUMN IF EXISTS input_unit_price,
  DROP COLUMN IF EXISTS cache_read_tokens;
DROP TABLE IF EXISTS cost_price;

-- Robust generation-cost attribution.
-- cost_line_item historically has no batch reference, so run cost is inferred by
-- matching request_id LIKE 'gen-<batch_id>-%'. Add a nullable batch_id for exact
-- attribution; the application writes it for new rows while readers keep the
-- request-id fallback so existing rows remain correct.
ALTER TABLE cost_line_item ADD COLUMN IF NOT EXISTS batch_id BIGINT;

CREATE INDEX IF NOT EXISTS idx_cost_line_item_batch
  ON cost_line_item(team_id, batch_id, biz_point)
  WHERE batch_id IS NOT NULL;

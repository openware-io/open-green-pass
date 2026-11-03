DROP INDEX IF EXISTS idx_cost_line_item_batch;
ALTER TABLE cost_line_item DROP COLUMN IF EXISTS batch_id;

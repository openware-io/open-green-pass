-- Bind each generation batch to the catalog service selected by the operator.
-- The repository is the source root; source_path narrows the input to the
-- registered service directory within that repository.
ALTER TABLE gen_batch ADD COLUMN service_id BIGINT REFERENCES tgt_service(id);
ALTER TABLE gen_batch ADD COLUMN source_path TEXT;
CREATE INDEX idx_gen_batch_service ON gen_batch(team_id, service_id, created_at DESC);

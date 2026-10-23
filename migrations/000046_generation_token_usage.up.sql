-- Token usage returned by the DSH gateway per service batch.
ALTER TABLE gen_batch ADD COLUMN IF NOT EXISTS tokens_in BIGINT NOT NULL DEFAULT 0;
ALTER TABLE gen_batch ADD COLUMN IF NOT EXISTS tokens_out BIGINT NOT NULL DEFAULT 0;

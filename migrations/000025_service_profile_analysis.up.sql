ALTER TABLE tgt_service
  ADD COLUMN IF NOT EXISTS profile_status TEXT NOT NULL DEFAULT 'pending'
    CHECK (profile_status IN ('pending', 'analyzing', 'suggested', 'confirmed', 'failed')),
  ADD COLUMN IF NOT EXISTS profile_source TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS profile_confidence NUMERIC(5,4),
  ADD COLUMN IF NOT EXISTS profile_evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
  ADD COLUMN IF NOT EXISTS test_strategy TEXT NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS profile_analyzed_at TIMESTAMPTZ;

UPDATE tgt_service
SET profile_status = CASE WHEN generation_status = 'confirmed' THEN 'confirmed' ELSE 'suggested' END
WHERE btrim(language) <> '' AND btrim(framework) <> '' AND btrim(test_tool) <> '' AND btrim(test_command) <> '';

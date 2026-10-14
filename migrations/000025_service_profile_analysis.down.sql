ALTER TABLE tgt_service
  DROP COLUMN IF EXISTS profile_analyzed_at,
  DROP COLUMN IF EXISTS test_strategy,
  DROP COLUMN IF EXISTS profile_evidence,
  DROP COLUMN IF EXISTS profile_confidence,
  DROP COLUMN IF EXISTS profile_source,
  DROP COLUMN IF EXISTS profile_status;

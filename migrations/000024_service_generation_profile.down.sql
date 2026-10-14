ALTER TABLE tgt_service
  DROP COLUMN IF EXISTS test_command,
  DROP COLUMN IF EXISTS test_tool,
  DROP COLUMN IF EXISTS framework,
  DROP COLUMN IF EXISTS language;

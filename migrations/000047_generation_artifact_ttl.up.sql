-- TTL cleanup for disposable generation artifacts. The batch tombstone remains
-- for audit; candidates and source bodies are removed after retention.
CREATE OR REPLACE FUNCTION gp.purge_generation_artifacts(cutoff TIMESTAMPTZ)
RETURNS BIGINT
LANGUAGE plpgsql
SECURITY DEFINER
AS $$
DECLARE
  removed BIGINT := 0;
  affected BIGINT := 0;
BEGIN
  DELETE FROM gen_seed s
  USING gen_batch b
  WHERE s.batch_id = b.id
    AND b.state IN ('failed','rejected','rolled_back','cancelled')
    AND b.updated_at < cutoff;
  GET DIAGNOSTICS affected = ROW_COUNT;
  removed := removed + affected;

  DELETE FROM gen_source_snapshot s
  USING gen_batch b
  WHERE s.batch_id = b.id
    AND b.state IN ('failed','rejected','rolled_back','cancelled')
    AND b.updated_at < cutoff;
  GET DIAGNOSTICS affected = ROW_COUNT;
  removed := removed + affected;

  RETURN removed;
END;
$$;

REVOKE ALL ON FUNCTION gp.purge_generation_artifacts(TIMESTAMPTZ) FROM PUBLIC;

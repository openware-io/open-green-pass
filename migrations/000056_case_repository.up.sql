ALTER TABLE cas_case ADD COLUMN IF NOT EXISTS repo_id bigint;
CREATE INDEX IF NOT EXISTS idx_cas_case_repo ON cas_case (team_id, target_id, repo_id);
UPDATE cas_case c
SET repo_id = v.source_repo_id
FROM cas_version v
WHERE v.case_id = c.id
  AND v.version = c.current_version
  AND c.repo_id IS NULL
  AND v.source_repo_id IS NOT NULL;

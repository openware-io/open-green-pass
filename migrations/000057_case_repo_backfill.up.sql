-- Backfill the current admin test corpus to its source repository.
-- m3-* cases were generated from the pc-admin OpenAPI inventory;
-- GEN-* cases were generated from the approved admin batch.
WITH admin_repo AS (
    SELECT id, project_id
    FROM repo_repo
    WHERE url LIKE '%/open-chat-admin.git'
    LIMIT 1
)
UPDATE cas_case c
SET repo_id = admin_repo.id
FROM admin_repo
WHERE c.repo_id IS NULL
  AND c.target_id = admin_repo.project_id
  AND (c.code LIKE 'm3-%' OR c.code LIKE 'GEN-%');

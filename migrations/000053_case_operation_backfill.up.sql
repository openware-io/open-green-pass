-- Human-readable operation for generated API cases: METHOD PATH.
-- This keeps all coverage variants of the same endpoint together in the library.
WITH request_ops AS (
    SELECT
        c.id,
        v.script_json #>> '{request,method}' AS method,
        v.script_json #>> '{request,path}' AS path
    FROM cas_case c
    JOIN cas_version v ON v.case_id = c.id AND v.version = c.current_version
    WHERE c.operation = ''
      AND v.script_json IS NOT NULL
      AND v.script_json #>> '{request,path}' IS NOT NULL
)
UPDATE cas_case c
SET
    operation = CASE
        WHEN request_ops.method IS NULL OR request_ops.method = '' THEN request_ops.path
        ELSE request_ops.method || ' ' || request_ops.path
    END,
    feature = CASE WHEN c.feature = '' THEN c.module ELSE c.feature END
FROM request_ops
WHERE c.id = request_ops.id;

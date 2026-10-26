DROP INDEX IF EXISTS idx_cas_case_taxonomy;
ALTER TABLE cas_case
    DROP COLUMN IF EXISTS business_domain,
    DROP COLUMN IF EXISTS feature,
    DROP COLUMN IF EXISTS module,
    DROP COLUMN IF EXISTS operation,
    DROP COLUMN IF EXISTS case_type,
    DROP COLUMN IF EXISTS priority,
    DROP COLUMN IF EXISTS automation_status;

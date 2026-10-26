DROP INDEX IF EXISTS idx_cas_case_review;
ALTER TABLE cas_case
    DROP COLUMN IF EXISTS requirement_ref,
    DROP COLUMN IF EXISTS defect_ref,
    DROP COLUMN IF EXISTS review_status,
    DROP COLUMN IF EXISTS review_comment;

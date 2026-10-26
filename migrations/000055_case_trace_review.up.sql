ALTER TABLE cas_case
    ADD COLUMN IF NOT EXISTS requirement_ref text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS defect_ref text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS review_status text NOT NULL DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS review_comment text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_cas_case_review ON cas_case (team_id, target_id, review_status);

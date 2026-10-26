-- Test-case taxonomy used by QA case management:
-- business domain -> feature -> module -> operation, plus type/priority/automation.
ALTER TABLE cas_case
    ADD COLUMN IF NOT EXISTS business_domain text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS feature text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS module text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS operation text NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS case_type text NOT NULL DEFAULT 'api',
    ADD COLUMN IF NOT EXISTS priority text NOT NULL DEFAULT 'P2',
    ADD COLUMN IF NOT EXISTS automation_status text NOT NULL DEFAULT 'automated';

CREATE INDEX IF NOT EXISTS idx_cas_case_taxonomy
    ON cas_case (team_id, target_id, business_domain, feature, module, operation);

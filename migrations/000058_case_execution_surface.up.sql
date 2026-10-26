ALTER TABLE cas_case ADD COLUMN IF NOT EXISTS execution_surface text NOT NULL DEFAULT 'api';
ALTER TABLE cas_case DROP CONSTRAINT IF EXISTS cas_case_execution_surface_check;
ALTER TABLE cas_case ADD CONSTRAINT cas_case_execution_surface_check CHECK (execution_surface IN ('api', 'browser', 'android', 'ios'));
CREATE INDEX IF NOT EXISTS idx_cas_case_surface ON cas_case (team_id, target_id, execution_surface);
-- Existing m3-* cases are API contract probes, never browser UI evidence.
UPDATE cas_case SET execution_surface = 'api' WHERE code LIKE 'm3-%';
UPDATE cas_case SET execution_surface = 'browser' WHERE execution_surface = 'api' AND kind IN ('web', 'ui');
UPDATE cas_case SET execution_surface = 'android' WHERE execution_surface = 'api' AND kind = 'mobile';

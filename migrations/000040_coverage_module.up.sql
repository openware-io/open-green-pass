-- M1: expose endpoint module on coverage cells for module-level planning.
ALTER TABLE cov_cell ADD COLUMN module TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_cov_cell_module ON cov_cell(team_id, project_id, module, status);

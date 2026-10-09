ALTER TABLE mdl_model ADD COLUMN protocol TEXT NOT NULL DEFAULT 'api' CHECK (protocol IN ('api','cc'));
ALTER TABLE mdl_model ADD COLUMN base_url TEXT NOT NULL DEFAULT '';
ALTER TABLE mdl_model ADD COLUMN credential_status TEXT NOT NULL DEFAULT 'missing' CHECK (credential_status IN ('missing','configured','verified','invalid'));
ALTER TABLE mdl_model ADD COLUMN configured_at TIMESTAMPTZ;

DROP POLICY IF EXISTS mdl_model_team_policy ON mdl_model;
CREATE POLICY mdl_model_team_policy ON mdl_model
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

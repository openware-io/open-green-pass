ALTER TABLE mdl_model DROP COLUMN IF EXISTS configured_at;
ALTER TABLE mdl_model DROP COLUMN IF EXISTS credential_status;
ALTER TABLE mdl_model DROP COLUMN IF EXISTS base_url;
ALTER TABLE mdl_model DROP COLUMN IF EXISTS protocol;
DROP POLICY IF EXISTS mdl_model_team_policy ON mdl_model;
CREATE POLICY mdl_model_team_policy ON mdl_model USING (team_id = current_setting('app.team_id', true)::bigint);

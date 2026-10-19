-- Bounded assistant state. Sensitive prompt content stays in the team-owned
-- session/turn tables; aud_event receives hashes and semantic references only.
CREATE TABLE assistant_session (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  subject TEXT NOT NULL,
  page_context TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_assistant_session_team_subject ON assistant_session(team_id, subject, updated_at DESC);
CREATE TRIGGER trg_assistant_session_tenant BEFORE INSERT ON assistant_session FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE assistant_session ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_assistant_session_rls ON assistant_session USING (team_id = current_setting('gp.team_id', true)::bigint) WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE assistant_turn (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  session_id BIGINT NOT NULL REFERENCES assistant_session(id) ON DELETE CASCADE,
  role TEXT NOT NULL CHECK(role IN ('user','assistant')),
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_assistant_turn_session ON assistant_turn(team_id, session_id, created_at, id);
CREATE TRIGGER trg_assistant_turn_tenant BEFORE INSERT ON assistant_turn FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE assistant_turn ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_assistant_turn_rls ON assistant_turn USING (team_id = current_setting('gp.team_id', true)::bigint) WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

CREATE TABLE assistant_artifact (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  kind TEXT NOT NULL,
  priority TEXT NOT NULL CHECK(priority IN ('high','medium','low')),
  title TEXT NOT NULL,
  summary TEXT NOT NULL,
  recommendation TEXT NOT NULL,
  evidence JSONB NOT NULL DEFAULT '[]'::jsonb,
  status TEXT NOT NULL DEFAULT 'open' CHECK(status IN ('open','archived')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_assistant_artifact_team_created ON assistant_artifact(team_id, status, created_at DESC);
CREATE TRIGGER trg_assistant_artifact_tenant BEFORE INSERT ON assistant_artifact FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
ALTER TABLE assistant_artifact ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_assistant_artifact_rls ON assistant_artifact USING (team_id = current_setting('gp.team_id', true)::bigint) WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

-- Each issuance and completion is a new row. No update path exists.
CREATE TABLE assistant_action (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  session_id BIGINT NOT NULL REFERENCES assistant_session(id) ON DELETE CASCADE,
  action_kind TEXT NOT NULL,
  route TEXT NOT NULL DEFAULT '',
  target TEXT NOT NULL DEFAULT '',
  values JSONB NOT NULL DEFAULT '{}'::jsonb,
  status TEXT NOT NULL CHECK(status IN ('issued','completed','cancelled')),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_assistant_action_session ON assistant_action(team_id, session_id, created_at, id);
CREATE TRIGGER trg_assistant_action_tenant BEFORE INSERT ON assistant_action FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_assistant_action_append_only BEFORE UPDATE OR DELETE ON assistant_action FOR EACH ROW EXECUTE FUNCTION gp.reject_append_only_mutation();
ALTER TABLE assistant_action ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_assistant_action_rls ON assistant_action USING (team_id = current_setting('gp.team_id', true)::bigint) WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

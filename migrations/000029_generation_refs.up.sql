-- Repository references are refreshed server-side and are separate from the
-- deployment-version registry. A branch and a tag may legitimately share a
-- name, so they cannot be represented by repo_branch alone.
CREATE TABLE repo_ref (
  id BIGINT PRIMARY KEY,
  team_id BIGINT NOT NULL,
  repo_id BIGINT NOT NULL REFERENCES repo_repo(id) ON DELETE CASCADE,
  ref_type TEXT NOT NULL CHECK (ref_type IN ('branch','tag')),
  name TEXT NOT NULL CHECK (btrim(name) <> ''),
  commit_sha TEXT NOT NULL CHECK (btrim(commit_sha) <> ''),
  fetched_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_by BIGINT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_by BIGINT,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (repo_id, ref_type, name)
);
CREATE INDEX idx_repo_ref_repo_type_name ON repo_ref(team_id, repo_id, ref_type, name);
CREATE TRIGGER trg_repo_ref_tenant BEFORE INSERT ON repo_ref FOR EACH ROW EXECUTE FUNCTION gp.set_tenant();
CREATE TRIGGER trg_repo_ref_audit BEFORE UPDATE ON repo_ref FOR EACH ROW EXECUTE FUNCTION gp.set_audit();
ALTER TABLE repo_ref ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_repo_ref_rls ON repo_ref USING (team_id = current_setting('gp.team_id', true)::bigint);

-- branch is retained as the wire/storage-compatible reference name. ref_type
-- removes its old branch-only ambiguity; base_batch_id records the comparison
-- baseline selected for an iterative generation.
ALTER TABLE gen_batch
  ADD COLUMN ref_type TEXT NOT NULL DEFAULT 'branch' CHECK (ref_type IN ('branch','tag')),
  ADD COLUMN base_batch_id BIGINT REFERENCES gen_batch(id);
CREATE INDEX idx_gen_batch_baseline ON gen_batch(team_id, base_batch_id) WHERE base_batch_id IS NOT NULL;

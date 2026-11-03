-- gen_plan contains tenant-scoped request/estimate snapshots; FORCE RLS ensures
-- even the table owner path cannot bypass the tenant policy.
ALTER TABLE gen_plan FORCE ROW LEVEL SECURITY;

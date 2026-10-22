DROP POLICY IF EXISTS p_def_suggestion_rls ON def_suggestion;
DROP TRIGGER IF EXISTS trg_def_suggestion_tenant ON def_suggestion;
DROP TABLE IF EXISTS def_suggestion;

DROP POLICY IF EXISTS p_def_event_rls ON def_event;
DROP TRIGGER IF EXISTS trg_def_event_tenant ON def_event;
DROP TABLE IF EXISTS def_event;

DROP POLICY IF EXISTS p_def_finding_rls ON def_finding;
DROP TRIGGER IF EXISTS trg_def_finding_audit ON def_finding;
DROP TRIGGER IF EXISTS trg_def_finding_tenant ON def_finding;
DROP TABLE IF EXISTS def_finding;

DROP TRIGGER IF EXISTS trg_evd_evidence_append_only ON evd_evidence;
DROP POLICY IF EXISTS p_evd_evidence_rls ON evd_evidence;
DROP TRIGGER IF EXISTS trg_evd_evidence_tenant ON evd_evidence;
DROP TABLE IF EXISTS evd_evidence;

DROP POLICY IF EXISTS p_gen_job_event_rls ON gen_job_event;
DROP TRIGGER IF EXISTS trg_gen_job_event_tenant ON gen_job_event;
DROP TABLE IF EXISTS gen_job_event;

DROP POLICY IF EXISTS p_gen_job_rls ON gen_job;
DROP TRIGGER IF EXISTS trg_gen_job_audit ON gen_job;
DROP TRIGGER IF EXISTS trg_gen_job_tenant ON gen_job;
DROP TABLE IF EXISTS gen_job;

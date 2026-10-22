DROP POLICY IF EXISTS p_ctr_error_rls ON ctr_error;
DROP TRIGGER IF EXISTS trg_ctr_error_tenant ON ctr_error;
DROP TABLE IF EXISTS ctr_error;

DROP POLICY IF EXISTS p_ctr_parameter_rls ON ctr_parameter;
DROP TRIGGER IF EXISTS trg_ctr_parameter_tenant ON ctr_parameter;
DROP TABLE IF EXISTS ctr_parameter;

DROP POLICY IF EXISTS p_ctr_endpoint_rls ON ctr_endpoint;
DROP TRIGGER IF EXISTS trg_ctr_endpoint_audit ON ctr_endpoint;
DROP TRIGGER IF EXISTS trg_ctr_endpoint_tenant ON ctr_endpoint;
DROP TABLE IF EXISTS ctr_endpoint;

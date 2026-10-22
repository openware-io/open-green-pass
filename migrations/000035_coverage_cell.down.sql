DROP POLICY IF EXISTS p_cov_waiver_rls ON cov_waiver;
DROP TRIGGER IF EXISTS trg_cov_waiver_tenant ON cov_waiver;
DROP TABLE IF EXISTS cov_waiver;

DROP POLICY IF EXISTS p_cov_cell_rls ON cov_cell;
DROP TRIGGER IF EXISTS trg_cov_cell_audit ON cov_cell;
DROP TRIGGER IF EXISTS trg_cov_cell_tenant ON cov_cell;
DROP TABLE IF EXISTS cov_cell;

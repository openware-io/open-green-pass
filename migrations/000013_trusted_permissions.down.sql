-- GP3-07A 回退：撤销本迁移授予的权限与触发器。
-- 角色不在此处删除：它们可能已被 DBA bootstrap 或其他部署绑定。

REVOKE ALL ON TABLE aud_event, cost_line_item, gate_result, gate_rule
  FROM gp_server, gp_worker, gp_verifier, gp_report_reader, gp_trusted_writer;

DROP TRIGGER IF EXISTS trg_gate_result_append_only ON gate_result;
DROP TRIGGER IF EXISTS trg_cost_line_item_append_only ON cost_line_item;
DROP TRIGGER IF EXISTS trg_aud_event_append_only ON aud_event;
DROP FUNCTION IF EXISTS gp.reject_append_only_mutation();

ALTER TABLE aud_event NO FORCE ROW LEVEL SECURITY;
ALTER TABLE cost_line_item NO FORCE ROW LEVEL SECURITY;
ALTER TABLE gate_result NO FORCE ROW LEVEL SECURITY;
ALTER TABLE gate_rule NO FORCE ROW LEVEL SECURITY;

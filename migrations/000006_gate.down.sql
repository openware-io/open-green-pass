-- GP1-05 回退：撤销门禁与审计哈希链表
DROP TABLE IF EXISTS gate_result;
DROP TABLE IF EXISTS gate_rule;
DROP TABLE IF EXISTS aud_event;
DROP ROLE IF EXISTS gp_trusted_writer;

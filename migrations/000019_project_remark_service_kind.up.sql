ALTER TABLE tgt_target ADD COLUMN remark TEXT NOT NULL DEFAULT '';

UPDATE tgt_service SET kind = 'gateway' WHERE name = 'gateway';
UPDATE tgt_service SET kind = 'websocket_service' WHERE name = 'im-access-ws';

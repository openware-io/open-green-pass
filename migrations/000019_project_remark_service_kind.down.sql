UPDATE tgt_service SET kind = 'api_service' WHERE kind IN ('gateway', 'websocket_service');
ALTER TABLE tgt_target DROP COLUMN IF EXISTS remark;

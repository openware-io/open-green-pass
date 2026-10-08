DROP FUNCTION IF EXISTS gp.claim_cicd_outbox(timestamptz);
DROP FUNCTION IF EXISTS gp.reclaim_cicd_outbox(timestamptz, interval);
DROP TABLE IF EXISTS cicd_outbox;

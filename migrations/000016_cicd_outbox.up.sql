CREATE TABLE cicd_outbox (
  id bigserial PRIMARY KEY,
  team_id bigint NOT NULL,
  event_id text NOT NULL,
  delivery_key text NOT NULL,
  provider text NOT NULL,
  attempt integer NOT NULL DEFAULT 0 CHECK (attempt >= 0),
  next_attempt_at timestamptz NOT NULL DEFAULT now(),
  status text NOT NULL CHECK (status IN ('pending','dispatching','delivered','failed')),
  last_error text NOT NULL DEFAULT '',
  payload jsonb NOT NULL DEFAULT '{}'::jsonb,
  claimed_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (team_id, delivery_key)
);
CREATE INDEX idx_cicd_outbox_due ON cicd_outbox(status, next_attempt_at, id);
CREATE INDEX idx_cicd_outbox_team ON cicd_outbox(team_id, created_at DESC);
ALTER TABLE cicd_outbox ENABLE ROW LEVEL SECURITY;
CREATE POLICY p_cicd_outbox_rls ON cicd_outbox
  USING (team_id = current_setting('gp.team_id', true)::bigint)
  WITH CHECK (team_id = current_setting('gp.team_id', true)::bigint);

-- A worker must claim the next due event across tenants, while all normal
-- request-path writes remain subject to RLS. This narrowly scoped definer
-- function is the sole cross-tenant claim boundary.
CREATE OR REPLACE FUNCTION gp.claim_cicd_outbox(p_now timestamptz)
RETURNS TABLE(team_id bigint, event_id text, delivery_key text, provider text,
  attempt integer, next_attempt_at timestamptz, status text, last_error text, payload jsonb)
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
  WITH next AS (
    SELECT id FROM public.cicd_outbox
    WHERE status = 'pending' AND next_attempt_at <= p_now
    ORDER BY next_attempt_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT 1
  )
  UPDATE public.cicd_outbox o
  SET status='dispatching', attempt=o.attempt+1, claimed_at=p_now, updated_at=now()
  FROM next WHERE o.id=next.id
  RETURNING o.team_id,o.event_id,o.delivery_key,o.provider,o.attempt,o.next_attempt_at,o.status,o.last_error,o.payload;
$$;
REVOKE ALL ON FUNCTION gp.claim_cicd_outbox(timestamptz) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION gp.claim_cicd_outbox(timestamptz) TO gp_worker;

CREATE OR REPLACE FUNCTION gp.reclaim_cicd_outbox(p_now timestamptz, p_timeout interval)
RETURNS integer
LANGUAGE sql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
  WITH reclaimed AS (
    UPDATE public.cicd_outbox
    SET status='pending', next_attempt_at=p_now, updated_at=now()
    WHERE status='dispatching' AND claimed_at IS NOT NULL AND claimed_at <= p_now - p_timeout
    RETURNING 1
  ) SELECT count(*)::integer FROM reclaimed;
$$;
REVOKE ALL ON FUNCTION gp.reclaim_cicd_outbox(timestamptz, interval) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION gp.reclaim_cicd_outbox(timestamptz, interval) TO gp_worker;

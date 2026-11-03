-- Cross-process wake-up for generation run SSE.
-- gen_run_event stays append-only; this trigger only emits a NOTIFY on commit so
-- every gp-server replica can fan the hint out to its own SSE subscribers.
-- Payload is the run id (opaque digits); consumers still read the authoritative
-- event rows with after_seq, so a dropped notification only delays delivery
-- until the low-frequency fallback poll.
CREATE OR REPLACE FUNCTION gp.notify_gen_run_event() RETURNS trigger AS $$
BEGIN
  PERFORM pg_notify('gp_gen_run_event', NEW.run_id::text);
  RETURN NEW;
END
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_gen_run_event_notify
  AFTER INSERT ON gen_run_event
  FOR EACH ROW EXECUTE FUNCTION gp.notify_gen_run_event();

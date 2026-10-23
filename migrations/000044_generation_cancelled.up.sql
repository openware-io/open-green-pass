-- Allow cancelled as a terminal generation batch state.
ALTER TABLE gen_batch DROP CONSTRAINT IF EXISTS gen_batch_state_check;
ALTER TABLE gen_batch
  ADD CONSTRAINT gen_batch_state_check
  CHECK (state IN ('enqueued','fetching','parsing','generating','quality_check','review','committing','approved','rejected','rolled_back','failed','cancelled'));

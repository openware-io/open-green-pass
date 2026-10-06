CREATE TABLE res_execution_pool (
  id text PRIMARY KEY,
  cluster_name text NOT NULL,
  resources text[] NOT NULL,
  endpoint text NOT NULL DEFAULT '',
  capacity integer NOT NULL CHECK (capacity > 0),
  available integer NOT NULL CHECK (available >= 0 AND available <= capacity),
  enabled boolean NOT NULL DEFAULT true,
  generation bigint NOT NULL DEFAULT 1 CHECK (generation > 0),
  last_heartbeat timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE res_res_execution_pool_lease (
  token uuid PRIMARY KEY,
  pool_id text NOT NULL REFERENCES res_execution_pool(id) ON DELETE CASCADE,
  units integer NOT NULL CHECK (units > 0),
  fencing_token bigint NOT NULL,
  expires_at timestamptz NOT NULL,
  released_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE (pool_id, fencing_token)
);

CREATE INDEX idx_res_execution_pool_live
  ON res_execution_pool (enabled, last_heartbeat DESC);
CREATE INDEX idx_res_res_execution_pool_lease_expiry
  ON res_res_execution_pool_lease (pool_id, expires_at)
  WHERE released_at IS NULL;

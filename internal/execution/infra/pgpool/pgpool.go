package pgpool

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	resourcepool "github.com/openware-io/open-green-pass/internal/execution/infra/pool"
)

var ErrStaleGeneration = errors.New("stale execution pool generation")
var ErrStaleLease = errors.New("stale execution pool lease")

type Registration struct {
	resourcepool.Registration
	Generation int64
}

type Lease struct {
	Token        string
	PoolID       string
	Units        int
	FencingToken int64
	ExpiresAt    time.Time
}

type Registry struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) (*Registry, error) {
	if pool == nil {
		return nil, errors.New("postgres execution pool is required")
	}
	return &Registry{pool: pool}, nil
}

// Register creates or takes ownership of a pool. Every takeover increments the
// generation, fencing heartbeats from an older agent process.
func (r *Registry) Register(ctx context.Context, registration resourcepool.Registration) (Registration, error) {
	if err := validateRegistration(registration); err != nil {
		return Registration{}, err
	}
	if registration.LastHeartbeat.IsZero() {
		registration.LastHeartbeat = time.Now().UTC()
	}
	const query = `INSERT INTO res_execution_pool
 (id, cluster_name, resources, endpoint, capacity, available, enabled, last_heartbeat)
 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
 ON CONFLICT (id) DO UPDATE SET
 cluster_name=EXCLUDED.cluster_name, resources=EXCLUDED.resources,
 endpoint=EXCLUDED.endpoint, capacity=EXCLUDED.capacity,
 available=LEAST(res_execution_pool.available, EXCLUDED.capacity), enabled=EXCLUDED.enabled,
 generation=res_execution_pool.generation+1, last_heartbeat=EXCLUDED.last_heartbeat, updated_at=now()
 RETURNING generation`
	var generation int64
	err := r.pool.QueryRow(ctx, query, registration.ID, registration.Cluster, registration.Resources,
		registration.Endpoint, registration.Capacity, registration.Available, registration.Enabled,
		registration.LastHeartbeat.UTC()).Scan(&generation)
	if err != nil {
		return Registration{}, fmt.Errorf("register execution pool: %w", err)
	}
	return Registration{Registration: registration, Generation: generation}, nil
}

func (r *Registry) Heartbeat(ctx context.Context, id string, generation int64, at time.Time) error {
	if strings.TrimSpace(id) == "" || generation <= 0 || at.IsZero() {
		return resourcepool.ErrInvalidHeartbeat
	}
	command, err := r.pool.Exec(ctx, `UPDATE res_execution_pool SET last_heartbeat=$3, updated_at=now()
 WHERE id=$1 AND generation=$2`, id, generation, at.UTC())
	if err != nil {
		return fmt.Errorf("heartbeat execution pool: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrStaleGeneration
	}
	return nil
}

// Reserve atomically selects a live capable pool, reclaims expired leases,
// deducts capacity and returns a monotonically increasing fencing token.
func (r *Registry) Reserve(ctx context.Context, resource string, units int, now time.Time, heartbeatTTL, leaseTTL time.Duration) (Lease, error) {
	if strings.TrimSpace(resource) == "" || units <= 0 || now.IsZero() || heartbeatTTL <= 0 || leaseTTL <= 0 {
		return Lease{}, resourcepool.ErrPoolUnavailable
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return Lease{}, fmt.Errorf("begin pool reservation: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()

	if _, err = tx.Exec(ctx, `WITH expired AS (
 UPDATE res_res_execution_pool_lease SET released_at=$1
 WHERE released_at IS NULL AND expires_at <= $1 RETURNING pool_id, units)
 UPDATE res_execution_pool p SET available=LEAST(p.capacity, p.available+x.units), updated_at=now()
 FROM (SELECT pool_id, sum(units)::integer units FROM expired GROUP BY pool_id) x WHERE p.id=x.pool_id`, now.UTC()); err != nil {
		return Lease{}, fmt.Errorf("reclaim expired pool leases: %w", err)
	}
	var lease Lease
	err = tx.QueryRow(ctx, `SELECT id FROM res_execution_pool
 WHERE enabled AND available >= $1 AND $2=ANY(resources) AND last_heartbeat >= $3
 ORDER BY available DESC, id FOR UPDATE SKIP LOCKED LIMIT 1`, units, resource, now.Add(-heartbeatTTL).UTC()).Scan(&lease.PoolID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Lease{}, resourcepool.ErrPoolNotFound
	}
	if err != nil {
		return Lease{}, fmt.Errorf("select execution pool: %w", err)
	}
	lease.Token = uuid.NewString()
	lease.Units = units
	lease.ExpiresAt = now.Add(leaseTTL).UTC()
	err = tx.QueryRow(ctx, `UPDATE res_execution_pool SET available=available-$2, updated_at=now()
 WHERE id=$1 RETURNING generation`, lease.PoolID, units).Scan(&lease.FencingToken)
	if err == nil {
		_, err = tx.Exec(ctx, `INSERT INTO res_res_execution_pool_lease(token,pool_id,units,fencing_token,expires_at)
 VALUES($1,$2,$3,$4,$5)`, lease.Token, lease.PoolID, lease.Units, lease.FencingToken, lease.ExpiresAt)
	}
	if err != nil {
		return Lease{}, fmt.Errorf("persist execution pool lease: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return Lease{}, fmt.Errorf("commit pool reservation: %w", err)
	}
	return lease, nil
}

func (r *Registry) Release(ctx context.Context, lease Lease) error {
	if lease.Token == "" || lease.PoolID == "" || lease.FencingToken <= 0 {
		return ErrStaleLease
	}
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin pool release: %w", err)
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	var units int
	err = tx.QueryRow(ctx, `UPDATE res_res_execution_pool_lease SET released_at=now()
 WHERE token=$1 AND pool_id=$2 AND fencing_token=$3 AND released_at IS NULL
 RETURNING units`, lease.Token, lease.PoolID, lease.FencingToken).Scan(&units)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrStaleLease
	}
	if err != nil {
		return fmt.Errorf("release execution pool lease: %w", err)
	}
	command, err := tx.Exec(ctx, `UPDATE res_execution_pool SET available=LEAST(capacity,available+$3), updated_at=now()
 WHERE id=$1 AND generation=$2`, lease.PoolID, lease.FencingToken, units)
	if err != nil {
		return fmt.Errorf("restore execution pool capacity: %w", err)
	}
	if command.RowsAffected() == 0 {
		return ErrStaleLease
	}
	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit pool release: %w", err)
	}
	return nil
}

// Renew extends a live lease only when its fencing token still belongs to the
// current pool generation. Expired or superseded leases cannot be resurrected.
func (r *Registry) Renew(ctx context.Context, lease Lease, now time.Time, leaseTTL time.Duration) (Lease, error) {
	if lease.Token == "" || lease.PoolID == "" || lease.FencingToken <= 0 || now.IsZero() || leaseTTL <= 0 {
		return Lease{}, ErrStaleLease
	}
	var renewed Lease
	renewed.Token, renewed.PoolID, renewed.Units, renewed.FencingToken = lease.Token, lease.PoolID, lease.Units, lease.FencingToken
	renewed.ExpiresAt = now.UTC().Add(leaseTTL)
	command, err := r.pool.Exec(ctx, `UPDATE res_res_execution_pool_lease l
SET expires_at=$4
FROM res_execution_pool p
WHERE l.token=$1 AND l.pool_id=$2 AND l.fencing_token=$3
  AND l.released_at IS NULL AND l.expires_at > $5
  AND p.id=l.pool_id AND p.generation=l.fencing_token`,
		lease.Token, lease.PoolID, lease.FencingToken, renewed.ExpiresAt, now.UTC())
	if err != nil {
		return Lease{}, fmt.Errorf("renew execution pool lease: %w", err)
	}
	if command.RowsAffected() == 0 {
		return Lease{}, ErrStaleLease
	}
	return renewed, nil
}

func validateRegistration(registration resourcepool.Registration) error {
	if registration.ID == "" || registration.Cluster == "" || registration.Capacity <= 0 || registration.Available < 0 || registration.Available > registration.Capacity || len(registration.Resources) == 0 {
		return resourcepool.ErrPoolUnavailable
	}
	return nil
}

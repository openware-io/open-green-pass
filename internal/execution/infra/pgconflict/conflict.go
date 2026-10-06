package pgconflict

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

var (
	ErrInvalidClaim  = errors.New("invalid resource claim")
	ErrClaimNotFound = errors.New("resource claim not found")
)

type heldClaim struct {
	key  int64
	conn *pgxpool.Conn
}

// Registry implements cross-process resource exclusion with PostgreSQL
// session advisory locks. Each active claim retains one pooled connection;
// PostgreSQL releases the lock automatically if its owner process exits.
type Registry struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	held map[string]heldClaim
}

func New(pool *pgxpool.Pool) (*Registry, error) {
	if pool == nil {
		return nil, errors.New("postgres conflict pool is required")
	}
	return &Registry{pool: pool, held: make(map[string]heldClaim)}, nil
}

func (r *Registry) Acquire(ctx context.Context, claim domain.ResourceClaim) (string, error) {
	if err := validateClaim(ctx, claim); err != nil {
		return "", err
	}
	if !claim.Exclusive {
		return "", nil
	}
	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return "", fmt.Errorf("acquire postgres conflict connection: %w", err)
	}
	key := lockKey(claim)
	var acquired bool
	if err := conn.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", key).Scan(&acquired); err != nil {
		conn.Release()
		return "", fmt.Errorf("acquire postgres conflict lock: %w", err)
	}
	if !acquired {
		conn.Release()
		return "", fmt.Errorf("%w: pool=%s resource=%s", domain.ErrResourceConflict, claim.PoolID, claim.ResourceType)
	}
	token, err := newToken()
	if err != nil {
		_, _ = conn.Exec(context.Background(), "SELECT pg_advisory_unlock($1)", key)
		conn.Release()
		return "", err
	}
	r.mu.Lock()
	r.held[token] = heldClaim{key: key, conn: conn}
	r.mu.Unlock()
	return token, nil
}

func (r *Registry) Release(ctx context.Context, token string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if token == "" {
		return nil
	}
	r.mu.Lock()
	claim, ok := r.held[token]
	if ok {
		delete(r.held, token)
	}
	r.mu.Unlock()
	if !ok {
		return ErrClaimNotFound
	}
	defer claim.conn.Release()
	var released bool
	if err := claim.conn.QueryRow(ctx, "SELECT pg_advisory_unlock($1)", claim.key).Scan(&released); err != nil {
		return fmt.Errorf("release postgres conflict lock: %w", err)
	}
	if !released {
		return ErrClaimNotFound
	}
	return nil
}

func validateClaim(ctx context.Context, claim domain.ResourceClaim) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if claim.TeamID <= 0 || claim.TargetID <= 0 || claim.OwnerID <= 0 || strings.TrimSpace(claim.ResourceType) == "" || strings.TrimSpace(claim.PoolID) == "" {
		return ErrInvalidClaim
	}
	return nil
}

func lockKey(claim domain.ResourceClaim) int64 {
	identity := strconv.FormatInt(claim.TeamID, 10) + "\x00" +
		strconv.FormatInt(claim.TargetID, 10) + "\x00" + claim.ResourceType + "\x00" + claim.PoolID
	sum := sha256.Sum256([]byte(identity))
	return int64(binary.BigEndian.Uint64(sum[:8]))
}

func newToken() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", fmt.Errorf("generate conflict token: %w", err)
	}
	return hex.EncodeToString(raw[:]), nil
}

var _ domain.ConflictPort = (*Registry)(nil)

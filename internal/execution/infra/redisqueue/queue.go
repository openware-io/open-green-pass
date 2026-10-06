package redisqueue

import (
	"context"
	"errors"
	"fmt"
	"github.com/openware-io/open-green-pass/internal/execution/domain"
	redis "github.com/redis/go-redis/v9"
	"strconv"
	"time"
)

type Queue struct {
	client        redis.UniversalClient
	prefix, group string
}

func New(ctx context.Context, c redis.UniversalClient, prefix, group string) (*Queue, error) {
	if c == nil || prefix == "" || group == "" {
		return nil, errors.New("redis queue client, prefix and group are required")
	}
	q := &Queue{c, prefix, group}
	e := c.XGroupCreateMkStream(ctx, q.stream(), group, "0").Err()
	if e != nil && !busy(e) {
		return nil, e
	}
	return q, nil
}
func (q *Queue) Enqueue(ctx context.Context, j domain.ScheduleJob) error {
	if j.ID == "" || j.TeamID <= 0 || j.Weight <= 0 || len(j.Payload) == 0 {
		return domain.ErrInvalidScheduleJob
	}
	if j.CreatedAt.IsZero() {
		j.CreatedAt = time.Now().UTC()
	}
	ok, e := q.client.SetNX(ctx, q.state(j.ID), "queued", 0).Result()
	if e != nil {
		return e
	}
	if !ok {
		return domain.ErrScheduleJobExists
	}
	_, e = q.client.HSet(ctx, q.job(j.ID), "id", j.ID, "team", j.TeamID, "weight", j.Weight, "payload", j.Payload, "created", j.CreatedAt.UnixNano()).Result()
	if e != nil {
		return e
	}
	score := float64(time.Now().UnixNano()) + 1/float64(j.Weight)
	return q.client.ZAdd(ctx, q.wfq(), redis.Z{Score: score, Member: j.ID}).Err()
}
func (q *Queue) Promote(ctx context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, domain.ErrInvalidScheduleJob
	}
	xs, e := q.client.ZPopMin(ctx, q.wfq(), int64(limit)).Result()
	if e != nil {
		return 0, e
	}
	n := 0
	for _, x := range xs {
		v, e := q.client.HMGet(ctx, q.job(fmt.Sprint(x.Member)), "id", "team", "weight", "payload", "created").Result()
		if e != nil {
			return n, e
		}
		if v[0] == nil {
			continue
		}
		_, e = q.client.XAdd(ctx, &redis.XAddArgs{Stream: q.stream(), Values: map[string]any{"id": fmt.Sprint(v[0]), "team": fmt.Sprint(v[1]), "weight": fmt.Sprint(v[2]), "payload": fmt.Sprint(v[3]), "created": fmt.Sprint(v[4])}}).Result()
		if e != nil {
			return n, e
		}
		q.client.Set(ctx, q.state(fmt.Sprint(x.Member)), "stream", 0)
		n++
	}
	return n, nil
}
func (q *Queue) Claim(ctx context.Context, c string, limit int, block time.Duration) ([]domain.ClaimedScheduleJob, error) {
	if c == "" || limit <= 0 {
		return nil, domain.ErrInvalidScheduleJob
	}
	s, e := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{Group: q.group, Consumer: c, Streams: []string{q.stream(), ">"}, Count: int64(limit), Block: block}).Result()
	if errors.Is(e, redis.Nil) {
		return nil, nil
	}
	if e != nil {
		return nil, e
	}
	var o []domain.ClaimedScheduleJob
	for _, st := range s {
		x, e := decode(st.Messages)
		if e != nil {
			return nil, e
		}
		o = append(o, x...)
	}
	return o, nil
}
func (q *Queue) Recover(ctx context.Context, c string, limit int, idle time.Duration) ([]domain.ClaimedScheduleJob, error) {
	if c == "" || limit <= 0 {
		return nil, domain.ErrInvalidScheduleJob
	}
	m, _, e := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{Stream: q.stream(), Group: q.group, Consumer: c, MinIdle: idle, Start: "0-0", Count: int64(limit)}).Result()
	if e != nil {
		return nil, e
	}
	return decode(m)
}
func (q *Queue) Ack(ctx context.Context, jobs ...domain.ClaimedScheduleJob) error {
	if len(jobs) == 0 {
		return nil
	}
	p := q.client.TxPipeline()
	ids := make([]string, len(jobs))
	for i, j := range jobs {
		if j.ID == "" || j.DeliveryID == "" {
			return domain.ErrInvalidScheduleJob
		}
		ids[i] = j.DeliveryID
		p.Set(ctx, q.state(j.ID), "done", 0)
	}
	p.XAck(ctx, q.stream(), q.group, ids...)
	_, e := p.Exec(ctx)
	return e
}
func decode(ms []redis.XMessage) ([]domain.ClaimedScheduleJob, error) {
	o := make([]domain.ClaimedScheduleJob, 0, len(ms))
	for _, m := range ms {
		f := func(k string) string { return fmt.Sprint(m.Values[k]) }
		t, e := strconv.ParseInt(f("team"), 10, 64)
		if e != nil {
			return nil, e
		}
		w, e := strconv.Atoi(f("weight"))
		if e != nil {
			return nil, e
		}
		n, e := strconv.ParseInt(f("created"), 10, 64)
		if e != nil {
			return nil, e
		}
		o = append(o, domain.ClaimedScheduleJob{ScheduleJob: domain.ScheduleJob{ID: f("id"), TeamID: t, Weight: w, Payload: []byte(f("payload")), CreatedAt: time.Unix(0, n).UTC()}, DeliveryID: m.ID})
	}
	return o, nil
}
func (q *Queue) wfq() string            { return q.prefix + ":wfq" }
func (q *Queue) stream() string         { return q.prefix + ":stream" }
func (q *Queue) job(id string) string   { return q.prefix + ":job:" + id }
func (q *Queue) state(id string) string { return q.prefix + ":state:" + id }
func busy(e error) bool                 { return len(e.Error()) >= 9 && e.Error()[:9] == "BUSYGROUP" }

var _ domain.ScheduleQueuePort = (*Queue)(nil)

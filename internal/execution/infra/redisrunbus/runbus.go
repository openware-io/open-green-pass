// Package redisrunbus provides cross-process run change notifications.
package redisrunbus

import (
	"context"
	"errors"
	"fmt"
	"sync"

	redis "github.com/redis/go-redis/v9"

	"github.com/openware-io/open-green-pass/internal/execution/domain"
)

type Bus struct{ client *redis.Client }

func New(ctx context.Context, address, password string) (*Bus, error) {
	if address == "" {
		return nil, errors.New("redis address is required")
	}
	client := redis.NewClient(&redis.Options{Addr: address, Password: password})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Bus{client: client}, nil
}

func (b *Bus) Publish(ctx context.Context, teamID, runID int64) error {
	return b.client.Publish(ctx, channel(teamID, runID), "changed").Err()
}

func (b *Bus) Subscribe(ctx context.Context, teamID, runID int64) (domain.RunEventSubscription, error) {
	pubsub := b.client.Subscribe(ctx, channel(teamID, runID))
	if _, err := pubsub.Receive(ctx); err != nil {
		_ = pubsub.Close()
		return nil, err
	}
	sub := &subscription{pubsub: pubsub, events: make(chan struct{}, 1), done: make(chan struct{})}
	go sub.forward(ctx)
	return sub, nil
}

func (b *Bus) Close() error { return b.client.Close() }

func channel(teamID, runID int64) string {
	return fmt.Sprintf("gp:run-events:%d:%d", teamID, runID)
}

type subscription struct {
	pubsub *redis.PubSub
	events chan struct{}
	done   chan struct{}
	once   sync.Once
}

func (s *subscription) forward(ctx context.Context) {
	defer close(s.events)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.done:
			return
		case _, ok := <-s.pubsub.Channel():
			if !ok {
				return
			}
			select {
			case s.events <- struct{}{}:
			default:
			}
		}
	}
}

func (s *subscription) Events() <-chan struct{} { return s.events }
func (s *subscription) Close() error {
	var err error
	s.once.Do(func() { close(s.done); err = s.pubsub.Close() })
	return err
}

var _ domain.RunEventBus = (*Bus)(nil)

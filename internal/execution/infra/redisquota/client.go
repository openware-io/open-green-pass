package redisquota

import (
	"context"
	"errors"

	redis "github.com/redis/go-redis/v9"
)

// ClientOptions contains connection data supplied by the composition root.
type ClientOptions struct {
	Address  string
	Password string
	DB       int
}

// Client adapts go-redis to the narrow evaluator boundary used by RedisQuota.
type Client struct {
	client *redis.Client
}

func NewClient(ctx context.Context, options ClientOptions) (*Client, error) {
	if options.Address == "" {
		return nil, errors.New("redis address is required")
	}
	client := redis.NewClient(&redis.Options{Addr: options.Address, Password: options.Password, DB: options.DB})
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return &Client{client: client}, nil
}

func (c *Client) Eval(ctx context.Context, script string, keys []string, args ...string) (any, error) {
	values := make([]any, len(args))
	for index := range args {
		values[index] = args[index]
	}
	return c.client.Eval(ctx, script, keys, values...).Result()
}

func (c *Client) Close() error { return c.client.Close() }

var _ Evaluator = (*Client)(nil)

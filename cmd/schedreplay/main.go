// Command schedreplay requeues one reviewed Redis Streams schedule dead-letter.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	redis "github.com/redis/go-redis/v9"

	"github.com/openware-io/open-green-pass/internal/execution/infra/redisqueue"
	"github.com/openware-io/open-green-pass/internal/platform/config"
)

func main() {
	var deliveryID, payloadFile string
	flag.StringVar(&deliveryID, "delivery-id", "", "Redis Stream ID in gp:execution:schedule:dead-letter")
	flag.StringVar(&payloadFile, "payload-file", "", "corrected JSON payload file; required")
	flag.Parse()
	if deliveryID == "" || payloadFile == "" {
		fmt.Fprintln(os.Stderr, "usage: gp-schedreplay --delivery-id <stream-id> --payload-file <corrected-payload.json>")
		os.Exit(2)
	}
	payload, err := os.ReadFile(payloadFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read corrected payload: %v\n", err)
		os.Exit(2)
	}
	cfg := config.Load()
	client := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr, Password: os.Getenv("GP_REDIS_PASSWORD")})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	queue, err := redisqueue.New(ctx, client, "gp:execution:schedule", "gp-execution")
	if err == nil {
		err = queue.ReplayDeadLetter(ctx, deliveryID, payload)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "replay scheduled dead-letter: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("replayed schedule dead-letter %s\n", deliveryID)
}

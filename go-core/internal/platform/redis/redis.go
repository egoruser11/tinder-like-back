package redis

import (
	"context"
	"time"

	redislib "github.com/redis/go-redis/v9"
)

func New(addr string) (*redislib.Client, error) {
	client := redislib.NewClient(&redislib.Options{Addr: addr})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return client, nil
}

package storage

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
)

type cache struct {
	*redis.Client
}

func newCache(conn *redis.Client) *cache {
	return &cache{
		conn,
	}
}

func (c *cache) get(ctx context.Context, key string, value interface{}) error {
	str, err := c.Get(ctx, key).Result()
	if err != nil {
		// returns err redis.Nil if key does not exist
		return err
	}

	return json.Unmarshal([]byte(str), value)
}

func (c *cache) set(ctx context.Context, key string, value interface{}, expiration int) error {
	str, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.Set(ctx, key, str, time.Duration(expiration)*time.Second).Err()
}

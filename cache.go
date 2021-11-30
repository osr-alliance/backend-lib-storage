package storage

import (
	"context"
	"encoding/json"
	"fmt"
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
		/*
			if err == redis.Nil {
				return nil
			}
		*/
		return err
	}

	return json.Unmarshal([]byte(str), value)
}

func (c *cache) set(ctx context.Context, key string, value interface{}, expiration int) error {
	fmt.Printf("setting cache with:\nKey: %s\nValue: %+v\n", key, value)
	str, err := json.Marshal(value)
	if err != nil {
		return err
	}

	return c.Set(ctx, key, str, time.Duration(expiration)*time.Second).Err()
}

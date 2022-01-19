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
	d("set() key: %s\n value: %+v\n", key, string(str))
	if err != nil {
		return err
	}

	return c.Set(ctx, key, str, time.Duration(expiration)*time.Second).Err()
}

func (c *cache) getList(ctx context.Context, q *Query, objMap map[string]interface{}, dest interface{}, opts *SelectOptions) error {
	d("getList")
	keyName := q.getKeyNameSelectOpts(objMap, opts)
	keyNameMetadata := q.getKeyNameMetadata(objMap)

	/*
		There's an invalidation issue where if the metadata key gets deleted (expired) then this key might be out of date as well.
		Check to see if the metadata key exists first and if not then throw a redis.Nil
	*/
	exists, err := c.Exists(ctx, keyNameMetadata).Result()
	if err != nil {
		return err
	}
	if exists == 0 {
		return redis.Nil
	}

	err = c.get(ctx, keyName, dest)
	if err != nil {
		return err // most likely a redis.Nil
	}

	d("getList() doing setList check")
	go func() {
		// new ctx so we don't have any cancellations
		c.setList(q, objMap, dest, opts)
	}()
	return nil
}

// setList updates the list's metadata to make sure it's up to date. This is idempotent
func (c *cache) setList(q *Query, objMap map[string]interface{}, dest interface{}, opts *SelectOptions) error {
	ctx := context.Background()

	d("setList")
	keyNameMetadata := q.getKeyNameMetadata(objMap)
	keyName := q.getKeyNameSelectOpts(objMap, opts)
	d("setList() keyNameMetadata: %s\n keyName: %s\n", keyNameMetadata, keyName)

	// first and foremost, set the key. Note: if this is being called from getLists then setting this is ok because we update the TTL
	c.set(ctx, keyName, dest, q.CacheTTL)

	d("setList() checking if exists")
	exists, err := c.Exists(ctx, keyNameMetadata).Result()
	if err != nil {
		d("setList() error: %+v", err)
		return err
	}

	// if the key doesn't exist, we need to create it & just push
	if exists == 0 {
		d("setList() key doesn't exist, so we're going to create it and push")
		return c.RPush(ctx, keyNameMetadata, keyName).Err()
	}

	d("setList() key exists, so we're going to update it")
	_, err = c.LPos(ctx, keyNameMetadata, keyName, redis.LPosArgs{}).Result()
	if err != nil {
		if err == redis.Nil {
			d("setList() key doesn't exist in the metadata, so we're going to create it and push")
			return c.RPush(ctx, keyNameMetadata, keyName).Err()
		}
	}
	return err
}

func (c *cache) updateList(q *Query, objMap map[string]interface{}) error {
	d("updateList")
	// As Logan says: deleting the key is never the wrong move.
	ctx := context.Background()
	keyNameMeta := q.getKeyNameMetadata(objMap)
	res, err := c.LRange(ctx, keyNameMeta, 0, -1).Result()
	if err != nil {
		return err
	}

	res = append(res, keyNameMeta)
	d("updateList() deleting: %+v", res)
	return c.Del(ctx, res...).Err()
}

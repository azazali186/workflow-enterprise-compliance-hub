package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type redisCache struct {
	client *redis.Client
}

func newRedis(url string) (*redisCache, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	return &redisCache{client: redis.NewClient(opts)}, nil
}

func (r *redisCache) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

func (r *redisCache) Get(ctx context.Context, key string) (string, bool) {
	v, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	return v, true
}

func (r *redisCache) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return r.client.Set(ctx, key, value, ttl).Err()
}

func (r *redisCache) SetNX(ctx context.Context, key, value string, ttl time.Duration) (bool, error) {
	ok, err := r.client.SetNX(ctx, key, value, ttl).Result()
	return ok, err
}

func (r *redisCache) Del(ctx context.Context, key string) error {
	return r.client.Del(ctx, key).Err()
}

func (r *redisCache) TTL(ctx context.Context, key string) (time.Duration, bool) {
	d, err := r.client.TTL(ctx, key).Result()
	if err != nil || d < 0 {
		return 0, false
	}
	return d, true
}

func (r *redisCache) Keys(ctx context.Context, pattern string) ([]string, error) {
	return r.client.Keys(ctx, pattern).Result()
}

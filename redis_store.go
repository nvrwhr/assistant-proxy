package main

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// RedisStore implements Memory using a Redis list per thread.
type RedisStore struct {
	rdb *redis.Client
}

func NewRedisStore(addr string) (*RedisStore, error) {
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &RedisStore{rdb: rdb}, nil
}

func (s *RedisStore) SaveMessage(threadID string, msg string) error {
	return s.rdb.RPush(context.Background(), threadID, msg).Err()
}

func (s *RedisStore) GetMessages(threadID string) ([]string, error) {
	return s.rdb.LRange(context.Background(), threadID, 0, -1).Result()
}

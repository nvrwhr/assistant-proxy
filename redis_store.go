package main

import (
	"context"
	"encoding/json"

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

func (s *RedisStore) SaveMessage(threadID string, msg Message) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return s.rdb.RPush(context.Background(), threadID, b).Err()
}

func (s *RedisStore) GetMessages(threadID string) ([]Message, error) {
	vals, err := s.rdb.LRange(context.Background(), threadID, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	msgs := make([]Message, 0, len(vals))
	for _, v := range vals {
		var m Message
		if err := json.Unmarshal([]byte(v), &m); err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
	}
	return msgs, nil
}

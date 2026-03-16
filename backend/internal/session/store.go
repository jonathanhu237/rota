package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrSessionNotFound = errors.New("session not found")

type Store struct {
	client  *redis.Client
	expires time.Duration
}

func NewStore(client *redis.Client, expires time.Duration) *Store {
	return &Store{
		client:  client,
		expires: expires,
	}
}

func (s *Store) Create(ctx context.Context, userID int64) (string, int64, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", 0, err
	}

	if err := s.client.Set(ctx, s.key(sessionID), strconv.FormatInt(userID, 10), s.expires).Err(); err != nil {
		return "", 0, err
	}

	return sessionID, int64(s.expires.Seconds()), nil
}

func (s *Store) Get(ctx context.Context, sessionID string) (int64, error) {
	value, err := s.client.Get(ctx, s.key(sessionID)).Result()
	if errors.Is(err, redis.Nil) {
		return 0, ErrSessionNotFound
	}
	if err != nil {
		return 0, err
	}

	userID, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse session user id: %w", err)
	}

	return userID, nil
}

func (s *Store) Refresh(ctx context.Context, sessionID string) (int64, error) {
	updated, err := s.client.Expire(ctx, s.key(sessionID), s.expires).Result()
	if err != nil {
		return 0, err
	}
	if !updated {
		return 0, ErrSessionNotFound
	}
	return int64(s.expires.Seconds()), nil
}

func (s *Store) Delete(ctx context.Context, sessionID string) error {
	return s.client.Del(ctx, s.key(sessionID)).Err()
}

func (s *Store) key(sessionID string) string {
	return "session:" + sessionID
}

func generateSessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

package rate_limiter_store

import (
    "context"
    "errors"
    "time"
)

var ErrKeyNotFound = errors.New("key not found")

type RateLimiterKey struct {
    UserId   string
    Endpoint string
}

type Store interface {
    // Get retrieves the value associated with the given key
    Get(ctx context.Context, key RateLimiterKey) (int32, error)
    // Set sets the value for the given key
    Set(ctx context.Context, key RateLimiterKey, timestamp time.Time, windowInterval, ttl time.Duration) error
}

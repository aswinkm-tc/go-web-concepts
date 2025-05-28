package rate_limiter_store

import (
    "context"
    "fmt"
    "github.com/mediocregopher/radix/v4"
    "time"
)

type redis struct {
    client    radix.Client
    scanCount int // Number of keys to scan in each iteration
}

func NewRedisStore(ctx context.Context, host string, scanCount int) (Store, error) {
    poolConfig := radix.PoolConfig{}
    c, err := poolConfig.New(ctx, "tcp", host)
    if err != nil {
        return nil, fmt.Errorf("failed to create Redis pool: %w", err)
    }
    return &redis{
        client:    c,
        scanCount: scanCount,
    }, nil
}

func generateKeyMatcher(key RateLimiterKey) string {
    return fmt.Sprintf("%s#%s#*", key.UserId, key.Endpoint)
}

func generateKey(key RateLimiterKey, timestampWindow time.Time) string {
    return fmt.Sprintf("%s#%s#%d", key.UserId, key.Endpoint, timestampWindow.Unix())
}

func (r *redis) Get(ctx context.Context, key RateLimiterKey) (int32, error) {
    var (
        k     string
        count int32
    )
    found := make(map[string]struct{})
    // Use a scanner to get all fields and values for the user at the given endpoint
    s := (radix.ScannerConfig{
        Pattern: generateKeyMatcher(key),
        Count:   r.scanCount,
        Type:    "string",
    }).New(r.client)
    for s.Next(ctx, &k) {
        if _, exists := found[k]; exists {
            // If the key has already been processed, skip it
            continue
        }
        var c int32
        if err := r.client.Do(ctx, radix.FlatCmd(&c, "GET", k)); err != nil {
            return 0, fmt.Errorf("failed to fetch rate limiter %s: %w", k, err)
        }
        count += c
        found[k] = struct{}{} // Mark this key as processed
    }
    return count, nil
}

// Set increments the request count for the user at the given timestamp by approximating the timestamp to the nearest
// redis.SlidingWindowInterval interval and sets the TTL for the key if it's a new time window.
func (r *redis) Set(ctx context.Context, key RateLimiterKey, timestamp time.Time, windowInterval, ttl time.Duration) error {
    // Calculate the boundary timestamp
    timestampWindow := timestamp.Truncate(windowInterval)

    // Use INCR to increment the count for the user at the boundary timestamp
    var count int32
    k := generateKey(key, timestampWindow)
    if err := r.client.Do(ctx, radix.FlatCmd(&count, "INCR", k)); err != nil {
        return fmt.Errorf("failed to set user %s for endpoint %s at %s: %w", key.UserId, key.Endpoint, timestampWindow, err)
    }

    if count == 1 {
        // Set the TTL for the key if this is a new timeWindow
        if err := r.client.Do(ctx, radix.FlatCmd(nil, "EXPIRE", k, int(ttl.Seconds()))); err != nil {
            return fmt.Errorf("failed to set TTL user %s for endpoint %s at  %s: %w", key.UserId, key.Endpoint, timestamp, err)
        }
    }

    return nil
}

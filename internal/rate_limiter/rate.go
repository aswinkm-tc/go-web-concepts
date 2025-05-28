package rate_limiter

import (
    "context"
    ratelimiterstore "github.com/aswinkm-tc/go-web-concepts/internal/rate_limiter_store"
    "github.com/cloudwego/hertz/pkg/app"
    "github.com/cloudwego/hertz/pkg/common/utils"
    "github.com/cloudwego/hertz/pkg/protocol/consts"
    "log/slog"
    "time"
)

// RateLimiter interface defines the methods for a rate limiter.
type RateLimiter interface {
    AllowRequest(ctx context.Context, endpoint, userId string) bool
    Middleware(ctx context.Context, c *app.RequestContext)
}

type EndpointConfig struct {
    // MaxRequests is the maximum number of requests allowed for the endpoint
    MaxRequests int `json:"max_requests"`
    // TimeWindow for the rate limit
    //
    // Defaults to 24 hours if not specified
    TimeWindow time.Duration `json:"time_window,omitempty"`
    // SlidingWindowInterval is used to round timestamps to the nearest boundary for rate limiting
    //
    // Defaults to 1 minute if not specified
    SlidingWindowInterval time.Duration `json:"sliding_window_interval,omitempty"`
}

// DefaultEndpointConfig returns the default configuration for an endpoint.
//
// Defaults are:
// - MaxRequests: 100
// - TimeWindow: 24 hours
// - SlidingWindowInterval: 1 minute
func DefaultEndpointConfig() EndpointConfig {
    return EndpointConfig{
        MaxRequests:           100,             // Default max requests
        TimeWindow:            24 * time.Hour,  // Default time window of 24 hours
        SlidingWindowInterval: 1 * time.Minute, // Default sliding window interval of 1 minute
    }
}

// RateLimiterConfig is a map of endpoint configurations for rate limiting.
type RateLimiterConfig map[string]EndpointConfig

// SanitizerFunc is a function type that sanitizes the path for rate limiting.
//
// It takes a byte slice representing the path and returns a sanitized path as a string.
type SanitizerFunc func(path []byte) string

type rateLimiter struct {
    config        RateLimiterConfig
    store         ratelimiterstore.Store // Store for persisting rate limiting data
    pathSanitizer SanitizerFunc          // Function to sanitize the path for rate limiting
}

// NewRateLimiter creates a new RateLimiter with the given configuration.
func NewRateLimiter(config RateLimiterConfig, store ratelimiterstore.Store, pathSanitizer SanitizerFunc) RateLimiter {
    c := &rateLimiter{
        config:        config,
        store:         store,
        pathSanitizer: pathSanitizer,
    }
    return c
}

// AllowRequest checks if a request is allowed for the given endpoint and user ID.
func (rl *rateLimiter) AllowRequest(ctx context.Context, endpoint string, userId string) bool {
    conf, ok := rl.config[endpoint]
    // If the endpoint is not configured for rate limiting, allow the request
    if !ok {
        return true
    }
    // Get the current timestamp
    curTimeStamp := time.Now()
    count, err := rl.store.Get(ctx, ratelimiterstore.RateLimiterKey{
        UserId:   userId,
        Endpoint: endpoint,
    })
    if err != nil {
        slog.Error("Error retrieving rate limiter object", "error", err)
        // If there is an error retrieving the rate limiter object, allow the request
        return true
    }
    if count == 0 {
        // If the key is not found, create a new rate limiter object with the current timestamp
        if err = rl.store.Set(ctx, ratelimiterstore.RateLimiterKey{
            UserId:   userId,
            Endpoint: endpoint,
        }, curTimeStamp, conf.SlidingWindowInterval, conf.TimeWindow); err != nil {
            slog.Error("Error setting rate limiter object", "error", err)
            // If there is an error setting the rate limiter object, allow the request
            return true
        }
        // Allow the request since this is the first request for this user and endpoint
        return true
    }

    if count < int32(conf.MaxRequests) {
        // If the sum of requests is less than the max allowed, allow the request
        if err = rl.store.Set(ctx, ratelimiterstore.RateLimiterKey{
            UserId:   userId,
            Endpoint: endpoint,
        }, curTimeStamp, conf.SlidingWindowInterval, conf.TimeWindow); err != nil {
            slog.Error("Error setting rate limiter object", "error", err)
            return true
        }
        return true
    }

    return false
}

func (rl *rateLimiter) Middleware(ctx context.Context, c *app.RequestContext) {
    endpoint := rl.pathSanitizer(c.Path()) // Get the endpoint from the request path
    ip := string(c.GetHeader("X-Forwarded-For"))
    if ip == "" {
        ip = c.ClientIP() // Fallback to the remote IP if X-Forwarded-For is not set
    }
    // Assume user_id is passed as a query parameter
    if !rl.AllowRequest(ctx, endpoint, ip) {
        c.JSON(consts.StatusTooManyRequests, utils.H{"error": "Rate limit exceeded"})
        c.Abort()
    }
    c.Next(ctx)
}

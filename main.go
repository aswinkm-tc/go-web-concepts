package main

import (
    "context"
    ratelimiter "github.com/aswinkm-tc/go-web-concepts/internal/rate_limiter"
    ratelimiterstore "github.com/aswinkm-tc/go-web-concepts/internal/rate_limiter_store"
    "github.com/cloudwego/hertz/pkg/app"
    "github.com/cloudwego/hertz/pkg/app/server"
    "github.com/cloudwego/hertz/pkg/common/utils"
    "github.com/cloudwego/hertz/pkg/protocol/consts"
    "strings"
    "time"
)

func main() {
    ctx := context.Background()

    // Create a Redis store for each endpoint with a TTL of 1 minute
    store, err := ratelimiterstore.NewRedisStore(ctx, "localhost:6379", 100) // 100 keys per scan
    if err != nil {
        panic(err)
    }

    rateLimiterDuration := 1 * time.Minute // Set the rate limit duration to 1 minute

    rateLimiterConfig := ratelimiter.RateLimiterConfig{
        "/": ratelimiter.DefaultEndpointConfig(),
        "/ping": ratelimiter.EndpointConfig{
            MaxRequests:           5,                   // Allow a maximum of 5 requests
            TimeWindow:            rateLimiterDuration, // Set the time window for the rate limit
            SlidingWindowInterval: 5 * time.Second,     // Set the sliding window interval to 1 second
        },
    }

    // Create a new rate limiter for the endpoint
    rateLimiter := ratelimiter.NewRateLimiter(rateLimiterConfig, store, sanitizePath)

    h := server.Default()
    // Register the rate limiter middleware for the endpoint
    h.Use(rateLimiter.Middleware)

    h.GET("/ping", func(ctx context.Context, c *app.RequestContext) {
        c.JSON(consts.StatusOK, utils.H{"message": "pong"})
    })
    h.GET("/", func(ctx context.Context, c *app.RequestContext) {
        c.JSON(consts.StatusOK, utils.H{"message": "Welcome to the rate limiter example!"})
    })

    h.Spin()
}

// sanitizePath only returns the first segment of the path, which is useful for rate limiting in this case.
//
// this function should be modified based on your application's routing structure for route matching
func sanitizePath(path []byte) string {
    // Remove leading and trailing slashes
    pathStr := strings.TrimSpace(string(path))
    pathStr = strings.Split(pathStr, "/")[1]
    return "/" + pathStr
}

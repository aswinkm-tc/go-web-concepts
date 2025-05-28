# Go Web Concepts

A collection of web application concepts implemented in Go, focusing on practical patterns and high-performance solutions.

## Table of Contents
- [Overview](#overview)
- [Technologies Used](#technologies-used)
- [Project Structure](#project-structure)
- [Rate Limiting](#rate-limiting)
- [Sliding Window Counter Algorithm](#sliding-window-counter-algorithm)
- [Considerations](#considerations)

## Overview
This repository demonstrates various web application concepts in Go. Each concept is implemented with clarity and extensibility in mind.

## Technologies Used
- [Go](https://golang.org/)
- [Hertz](https://github.com/cloudwego/hertz) — High-performance web framework

## Project Structure
```
go-web-concepts/
├── main.go
├── internal/
│   ├── rate_limiter/
│   │   └── rate.go
│   └── rate_limiter_store/
│       ├── redis.go
│       └── store.go
├── hack/
│   └── docker-compose.yaml
├── go.mod
├── go.sum
└── Readme.md
```

## Rate Limiting
Rate limiting is a technique used to control the amount of traffic sent or received by an application. In this project, rate limiting is applied to different routes using the Sliding Window Counter algorithm.

**Key Features:**
- Stores the timestamp of each request
- Counts the number of requests within a sliding time window
- Sanitizes request paths to ensure consistent rate limiting (ignores query parameters and variations)

## Sliding Window Counter Algorithm

**Function:** `AllowRequest(requestPath string, userId string) (isAllowed bool, err error)`

Checks if a new request is allowed:

1. Time is broken into fixed time buckets (you could use seconds, milliseconds, etc.)
2. For every request:
    - Check if the request is allowed based on the rate limit.
      - If `getCount(requestPath, userId) > maxRequests`:
        - Return failure response
    - Get the current second (timestamp).
    - Figure out the nearest bucket.
    - Increment the counter for that bucket.
    - Return success response.

**Function:** `getCount(requestPath string, userId string) (count int, err error)`

Finds the number of requests made by the user for the given path:

1. Get all the buckets for the given path and userId.
2. Filter the buckets to only include those that are within the sliding time window.
3. Count the number of requests in the filtered buckets.
4. Return the count.

Alternatively, you can set a TTL (Time To Live) for the buckets to automatically remove them after a certain period of time:
- This allows the rate limiter to automatically clean up old buckets and prevent memory leaks.
- This also allows the rate limiter to not filter the buckets every time a request is made, which can improve performance.

### Implementation

There are two Go packages which allow you to do this:
- **rate_limiter**: Contains a `RateLimiter` interface with two methods:
  - `AllowRequest`: Checks if a request is allowed based on the rate limit.
  - `Middleware`: A middleware function that can be used in HTTP handlers to enforce the rate limit. </br>
    This middleware is compatible with the hertz framework.

  The package also provides a `NewRateLimiter` function to create a new instance of a ratelimiter.
- **rate_limiter_store**: the rate_limiter_store package provides an interface for storing rate limit data.</br>
  It includes methods for getting and setting the rate limit data.</br>
  The package also provides a `NewRateLimiterStore` function to create a new instance of a rate limiter store.</br>

### Redis as a Store
I'm using redis as a store for the rate limiter data, but you can implement your own store by implementing the `RateLimiterStore` interface.</br>
The main advantage of using redis is that it allows you to set a TTL (Time To Live) for the keys
I'm using the following as the key format for the rate limiter data:
```
    <userId>#<requestPath>#<timestamp>
```
This allows to easily get the rate limit data for a specific user and request path.</br>
This also allows to easily set the TTL for the keys to automatically remove them after a certain period of which is the sliding time window.</br>

* Using INCR command to increment the count of requests for a specific bucket
* Using EXPIRE command to set the TTL for the keys to automatically remove them on each sliding time window
* Using SCAN command to get all the keys for a specific user and request path
  * While scanning the keys, we filter the keys to only include those that contain the request path and userId
  * This is done by adding a Matcher to the SCAN command that matches the request path and userId
  * By limiting the number of keys returned by the SCAN command, we can avoid fetching too many keys at once
* Using GET command to fetch the counter for a specific bucket once you have the keys

### Usage
The following example shows how to use the ratelimiter in a hertz application:
```go
    rateLimiterConfig := ratelimiter.RateLimiterConfig{
        "/": ratelimiter.DefaultEndpointConfig(),
        "/ping": ratelimiter.EndpointConfig{
            MaxRequests:           5,                   // Allow a maximum of 5 requests
            TimeWindow:            1 * time.Minute, // Set the time window for the rate limit
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
```

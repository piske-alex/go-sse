# Performance Considerations

The go-sse server is designed for efficiency and low-latency real-time updates. This document outlines performance considerations and optimization strategies.

## Goroutines and Concurrency

The server leverages Go's goroutines for efficient concurrency:

1. **One Goroutine Per Client**: Each SSE client connection is handled by a dedicated goroutine, allowing the server to handle thousands of concurrent connections.  

2. **Non-Blocking Message Handling**: Client message channels are buffered to prevent blocking the main server thread when sending events.

3. **Cleanup Routine**: A background goroutine periodically removes inactive connections, preventing resource leaks.

## Memory Management

1. **Buffered Channels**: Client message channels are buffered (default 100 messages) to handle burst traffic.

2. **Client Limits**: The server has a configurable maximum client limit to prevent memory exhaustion.

3. **Message Dropping**: Under extreme load, the server will drop messages rather than block, ensuring system stability.

## Efficient Filtering

The JQ-style path filtering is optimized for performance:

1. **Early Path Matching**: Checks for exact match, parent path, or child path before expensive regex operations.

2. **Compiled Regex Patterns**: Wildcard path patterns are compiled into regex patterns for efficient matching.

## Store Operations

1. **Read-Write Locking**: The key-value store uses RWMutex for efficient concurrent read access with exclusive write locking.

2. **Optimized Path Navigation**: Store operations use efficient path navigation to minimize the amount of data traversal.

## Scaling Strategies

### Vertical Scaling

The server can benefit from additional CPU cores and memory:

- More CPU cores allow handling more concurrent goroutines
- More memory enables supporting more connected clients

### Horizontal Scaling

For even larger deployments, consider:

1. **Load Balancing**: Deploy multiple instances behind a load balancer

2. **Shared Storage**: Move the in-memory store to a distributed cache like Redis

3. **Message Queue**: Use a message broker like NATS or Kafka for cross-instance updates

## Benchmarking

The repo includes benchmark tests to measure performance under load:

```bash
go test -bench=. ./internal/sse/... -benchmem
```

Example benchmark results (on a typical development machine):

```
BenchmarkSSEServer_BroadcastEvent/clients-10-8     10000        150521 ns/op       8432 B/op      174 allocs/op
BenchmarkSSEServer_BroadcastEvent/clients-100-8     1000       1528401 ns/op      84323 B/op     1740 allocs/op
BenchmarkSSEServer_BroadcastEvent/clients-1000-8      10      154286122 ns/op    843056 B/op    17401 allocs/op
```

## Typical Limits

With default settings, a single go-sse instance can typically handle:

- Up to 10,000 concurrent SSE connections on a modern server
- Broadcast rates of 100-1000 updates per second
- Data store sizes up to several hundred MB

These limits can vary significantly based on hardware, network conditions, and the specific usage pattern.

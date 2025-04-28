# go-sse

An efficient Go-based SSE (Server-Sent Events) server with JQ-style query support for an in-memory key-value store.

## Features

- Low-latency event delivery to clients
- Scalable connection handling using goroutines
- In-memory key-value store with JQ-style queries
- Client filtering capabilities
- HTTP API for store management

## Installation

```bash
go get github.com/piske-alex/go-sse
```

## Quick Start

```go
package main

import (
    "log"
    "github.com/piske-alex/go-sse/cmd/server"
)

func main() {
    server.Run(":8080")
}
```

## API Usage

### Establish an SSE Connection

```
GET /events?filter=.data.users[*].status
```

### Initialize KV Store

```
POST /store
Content-Type: application/json

{
  "data": {
    "users": [
      {"id": 1, "name": "Alice", "status": "online"},
      {"id": 2, "name": "Bob", "status": "offline"}
    ],
    "config": {
      "maxUsers": 100,
      "timeout": 30
    }
  }
}
```

### Update KV Store

```
PATCH /store?path=.data.users[0].status
Content-Type: application/json

"away"
```

### Query KV Store

```
GET /store?path=.data.users[*]
```

## License

MIT
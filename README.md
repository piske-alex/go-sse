# go-sse

An efficient Go-based SSE (Server-Sent Events) server with JQ-style query support for in-memory or MongoDB key-value store.

## Features

- Low-latency event delivery to clients
- Scalable connection handling using goroutines
- Support for MongoDB for large JSON document storage
- In-memory key-value store option for simpler deployments
- JQ-style queries for data access and filtering
- Client filtering capabilities
- HTTP API for store management
- Support for large POST requests (up to 20MB)

## Storage Options

### In-Memory Store

- Fast and simple for development and smaller deployments
- No external dependencies required
- Limited by available memory

### MongoDB Store

- Supports very large JSON documents (up to 16MB)
- Atomic operations on large documents
- Persistence across restarts
- Change stream integration for real-time updates
- Authentication support for secure deployments

## Installation

```bash
go get github.com/piske-alex/go-sse
```

## Quick Start

### Configuration

Create a `.env` file based on the example:

```bash
cp .env.example .env
```

Edit the `.env` file to configure your preferred storage option:

```
# Server configuration
PORT=8080

# Store configuration
# Options: memory, mongo
STORE_TYPE=mongo

# MongoDB configuration - Option 1: Connection string
MONGO_URI=mongodb://username:password@localhost:27017/gosse?authSource=admin

# MongoDB configuration - Option 2: Individual components
# MONGO_HOST=localhost
# MONGO_PORT=27017
# MONGO_USER=username
# MONGO_PASSWORD=password
# MONGO_AUTH_DB=admin

# MongoDB database settings
MONGO_DB_NAME=gosse
MONGO_COLLECTION=kv_store
MONGO_DOCUMENT_ID=main

# Request size limit
MAX_REQUEST_SIZE_MB=20
```

### Running with Docker Compose

```bash
# With in-memory store
export STORE_TYPE=memory
docker-compose up -d

# With MongoDB (uses default admin/password credentials)
export STORE_TYPE=mongo
docker-compose --profile with-mongo up -d

# With MongoDB using custom credentials
export STORE_TYPE=mongo MONGO_USER=myuser MONGO_PASSWORD=mypassword
docker-compose --profile with-mongo up -d

# With MongoDB and client example
export STORE_TYPE=mongo
docker-compose --profile with-mongo --profile with-client up -d
```

### Running Locally

```bash
go run ./cmd/server/main.go
```

Or as a custom application:

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

## Documentation

- [Using JQ-Style Paths](docs/using_jq_paths.md)
- [Performance Considerations](docs/performance.md)
- [Deployment Guide](docs/deployment.md)
- [MongoDB Setup](docs/mongodb_setup.md)
- [MongoDB Authentication](docs/mongodb_auth.md)
- [Deployment with Nixpacks](docs/deployment_with_nixpacks.md)

## Connection Capacity

The go-sse server can typically handle:

- **~10,000 concurrent SSE connections** on a standard server with 4-8GB RAM
- **~50,000+ concurrent connections** on more powerful hardware with proper tuning

The MongoDB integration allows for handling very large JSON documents (up to 16MB per document) with atomic operations.

## License

MIT

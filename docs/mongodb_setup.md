# MongoDB Integration for go-sse

This document explains how to set up and use go-sse with MongoDB for handling large JSON documents with atomic operations.

## Overview

Integrating MongoDB with go-sse provides several advantages for handling large JSON blobs:

1. **Atomic Operations**: MongoDB supports atomic operations on entire documents
2. **Large Document Support**: MongoDB can handle documents up to 16MB
3. **Query Capabilities**: MongoDB's query language works well with our JQ-style path expressions
4. **Change Streams**: MongoDB change streams allow real-time notifications of data changes

## Configuration

### Environment Variables

The following environment variables control the MongoDB integration:

- `STORE_TYPE`: Set to `mongo` to use MongoDB (default: `memory`)
- `MONGO_URI`: MongoDB connection string (default: `mongodb://localhost:27017`)
- `MONGO_DB_NAME`: MongoDB database name (default: `gosse`)
- `MONGO_COLLECTION`: MongoDB collection name (default: `kv_store`)
- `MONGO_DOCUMENT_ID`: ID for the main document (default: `main`)

You can set these variables in a `.env` file (copy from `.env.example`) or provide them as environment variables.

## Local Development with MongoDB

### Using Docker Compose

The easiest way to run go-sse with MongoDB is using Docker Compose:

```bash
# Start with MongoDB
export STORE_TYPE=mongo
docker-compose --profile with-mongo up -d

# Start with MongoDB and MongoDB Express admin interface
export STORE_TYPE=mongo
docker-compose --profile with-mongo up -d

# Start with MongoDB and test client
export STORE_TYPE=mongo
docker-compose --profile with-mongo --profile with-client up -d
```

### MongoDB Express Admin Interface

When running with `--profile with-mongo`, a MongoDB Express admin interface is available at http://localhost:8081. This allows you to view and edit the MongoDB documents directly.

## Document Structure

The MongoDB integration uses a simple document structure:

```json
{
  "_id": "main",
  "data": {
    // Your actual JSON data goes here
    "users": [...],
    "config": {...},
    // etc.
  }
}
```

All operations work on the `data` field of this document, which can contain arbitrarily complex JSON structures.

## Change Stream Detection

The go-sse server automatically sets up a MongoDB change stream to detect updates to the document. When changes are detected, the server broadcasts the updates to connected SSE clients based on their filter paths.

## Deployment Considerations

### MongoDB Atlas

For production, consider using MongoDB Atlas, a fully managed MongoDB service:

1. Create an account at [MongoDB Atlas](https://www.mongodb.com/cloud/atlas)
2. Create a new cluster
3. Configure network access and database users
4. Get the connection string and set it as `MONGO_URI`

### Replica Sets

MongoDB change streams require a replica set. If you're deploying your own MongoDB server:

1. Configure MongoDB as a replica set (even with one node)
2. Update the connection string to include the replica set name

### Security

For production deployments:

1. Use TLS/SSL connections to MongoDB
2. Create a dedicated user with minimal permissions
3. Use a strong password and store it securely
4. Limit network access to the MongoDB server

## Example: Initializing with a Large JSON Document

```bash
# Initialize the store with a large JSON document
curl -X POST http://localhost:8080/store \
  -H "Content-Type: application/json" \
  -d @large-document.json
```

## Example: Updating a Specific Path

```bash
# Update a specific path in the document
curl -X PATCH "http://localhost:8080/store?path=.users[0].status" \
  -H "Content-Type: application/json" \
  -d '"away"'
```

## Monitoring

To check the current store type and connection status:

```bash
curl http://localhost:8080/metrics
```

This will return information including the store type (memory or mongodb).

## Troubleshooting

### Connection Issues

If you encounter connection issues:

1. Check the MongoDB connection string
2. Verify network connectivity between go-sse and MongoDB
3. Check MongoDB logs for authentication errors
4. Ensure MongoDB is running as a replica set if using change streams

### Performance Issues

If you experience performance issues with large documents:

1. Monitor MongoDB performance using MongoDB Compass or the admin interface
2. Consider splitting very large documents into multiple smaller documents
3. Use MongoDB indexes if you have multiple documents
4. For extremely large JSON blobs (>10MB), consider sharding or alternative storage solutions

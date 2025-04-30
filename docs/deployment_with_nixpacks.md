# Deployment with Nixpacks

## Overview

The go-sse project uses Nixpacks as the primary deployment method, providing better support for:

1. Large JSON document processing (up to 20MB)
2. MongoDB integration for persistent storage
3. Simple but effective configuration

## Why Nixpacks?

We've chosen Nixpacks for deployment for several reasons:

1. **Better Platform Integration**: Nixpacks works seamlessly with platforms like Render, Railway, and Fly.io
2. **Simplified Environment Variables**: Better handling of environment variables for MongoDB configuration
3. **Automatic Optimization**: Nixpacks includes optimizations for Go applications without manual configuration
4. **Consistent Deployments**: More consistent behavior across different deployment platforms

## Direct HTTP Server Approach

Unlike many web applications that place a reverse proxy (like Nginx) in front of the application server, our go-sse implementation can efficiently handle HTTP requests directly. The Go HTTP server has been optimized to:

- Handle large POST requests (up to 20MB)
- Manage long-lived SSE connections efficiently
- Compress response data automatically
- Process requests concurrently using Go's goroutines

This direct approach simplifies the architecture and reduces complexity, while still maintaining high performance.

## Large POST Request Support

The go-sse server is configured to handle large JSON documents through environment variables:

```toml
[variables]
MAX_REQUEST_SIZE_MB = "20"   # Support POST requests up to 20MB
```

This allows the server to process large JSON documents when initializing or updating the store, which is essential for applications with complex data structures.

## MongoDB Configuration

The nixpacks.toml file includes environment variables for MongoDB integration:

```toml
[variables]
STORE_TYPE = "mongo"
MONGO_URI = "$MONGO_URI"
MONGO_DB_NAME = "$MONGO_DB_NAME"
MONGO_COLLECTION = "$MONGO_COLLECTION"
MONGO_DOCUMENT_ID = "$MONGO_DOCUMENT_ID"
```

These variables allow you to configure the MongoDB connection at deployment time, making it easy to connect to different MongoDB instances in different environments.

## How to Deploy

### Render

1. Connect your GitHub repository to Render
2. Select "Web Service"
3. Render will automatically detect the nixpacks.toml file
4. Configure the following environment variables:
   - `STORE_TYPE=mongo` (if using MongoDB)
   - `MONGO_URI=mongodb+srv://...` (your MongoDB connection string)
   - `MONGO_DB_NAME=your_db_name`
   - `MONGO_COLLECTION=your_collection`
   - `MONGO_DOCUMENT_ID=main` (or your preferred document ID)
   - `MAX_REQUEST_SIZE_MB=20` (or your preferred maximum size)

### Railway

1. Connect your GitHub repository to Railway
2. Railway will automatically detect the nixpacks.toml file
3. Configure the MongoDB environment variables as described above

### Fly.io

1. Install the Fly CLI: `curl -L https://fly.io/install.sh | sh`
2. Authenticate: `flyctl auth login`
3. Launch the app: `flyctl launch`
4. Set environment variables: `flyctl secrets set STORE_TYPE=mongo MONGO_URI=...`
5. Deploy: `flyctl deploy`

## Local Testing

You can test the nixpacks configuration locally:

```bash
# Install nixpacks
curl -sSL https://nixpacks.com/install.sh | bash

# Build the application
nixpacks build . -n go-sse

# Run with environment variables
docker run -p 8080:8080 \
  -e STORE_TYPE=mongo \
  -e MONGO_URI=mongodb://localhost:27017 \
  -e MONGO_DB_NAME=gosse \
  -e MONGO_COLLECTION=kv_store \
  -e MONGO_DOCUMENT_ID=main \
  -e MAX_REQUEST_SIZE_MB=20 \
  go-sse
```

## Troubleshooting

If you encounter issues with the nixpacks deployment:

1. Make sure your MongoDB URI is correct and accessible from your deployment environment
2. Check that you've set all required environment variables
3. Ensure the `MAX_REQUEST_SIZE_MB` is set appropriately for your data size

For more detailed troubleshooting, refer to the logs provided by your deployment platform.

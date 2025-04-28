# Deployment Guide for go-sse

This document provides instructions for deploying the go-sse server using various methods.

## Table of Contents

1. [Local Development](#local-development)
2. [Docker Deployment](#docker-deployment)
3. [Docker Compose Deployment](#docker-compose-deployment)
4. [Nixpacks Deployment](#nixpacks-deployment)
5. [Cloud Platform Deployment](#cloud-platform-deployment)

## Local Development

For local development and testing:

```bash
# Build the application
go build -o bin/go-sse ./cmd/server

# Run the application
./bin/go-sse
```

Alternatively, use the Makefile:

```bash
# Build and run
make run
```

## Docker Deployment

The repository includes a Dockerfile for containerized deployment:

```bash
# Build the Docker image
docker build -t go-sse .

# Run the container
docker run -p 8080:8080 go-sse
```

## Docker Compose Deployment

For a more complete setup including Nginx for SSL termination and a test client:

```bash
# Basic deployment (just the SSE server)
docker-compose up -d

# With Nginx for SSL termination
docker-compose --profile with-nginx up -d

# With test client
docker-compose --profile with-client up -d

# With both Nginx and test client
docker-compose --profile with-nginx --profile with-client up -d
```

### SSL Certificate Setup

Before running with Nginx, you need to create SSL certificates:

```bash
# Create the certs directory
mkdir -p nginx/certs

# Generate a self-signed certificate for testing
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout nginx/certs/server.key \
  -out nginx/certs/server.crt
```

For production, use proper certificates from a certificate authority.

## Nixpacks Deployment

The repository includes a `nixpacks.toml` file for deployment on platforms that support Nixpacks:

```bash
# Install nixpacks if you don't have it
curl -sSL https://nixpacks.com/install.sh | bash

# Build the application using nixpacks
nixpacks build . -n go-sse

# Run the built image
docker run -p 8080:8080 go-sse
```

## Cloud Platform Deployment

### Render

1. Connect your GitHub repository to Render
2. Select "Web Service"
3. Choose "Docker" as the Environment
4. Set the environment variable `PORT=8080`

### Fly.io

1. Install the Fly CLI: `curl -L https://fly.io/install.sh | sh`
2. Authenticate: `flyctl auth login`
3. Launch the app: `flyctl launch`
4. Deploy: `flyctl deploy`

### Railway

1. Connect your GitHub repository to Railway
2. Railway will automatically detect the nixpacks.toml and deploy your application

### Heroku

1. Install the Heroku CLI: `npm install -g heroku`
2. Login: `heroku login`
3. Create an app: `heroku create`
4. Deploy: `git push heroku main`

## Environment Variables

- `PORT`: The port on which the server will listen (default: 8080)

## Health Checks

The application exposes a `/health` endpoint that returns an HTTP 200 status code when the server is running correctly. Use this endpoint for health checks in your deployment platform.

## Scaling

For higher load scenarios:

1. Deploy multiple instances behind a load balancer
2. Consider using a distributed cache like Redis instead of the in-memory store
3. Use a message broker like NATS or Kafka for cross-instance updates

See the [Performance Considerations](performance.md) document for more information on scaling strategies.

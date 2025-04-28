# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git make gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go.mod and go.sum files first to leverage Docker layer caching
COPY go.mod ./
COPY go.sum ./

# Download dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o go-sse ./cmd/server

# Final stage
FROM alpine:latest

# Install certificates for HTTPS
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/go-sse .

# Copy examples directory (optional)
COPY --from=builder /app/examples ./examples

# Expose port
EXPOSE 8080

# Create a non-root user to run the application
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# Define environment variables
ENV PORT=8080

# Run the binary
CMD ["./go-sse"]

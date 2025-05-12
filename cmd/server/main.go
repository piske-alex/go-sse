package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/piske-alex/go-sse/internal/api"
	"github.com/piske-alex/go-sse/internal/sse"
	"github.com/piske-alex/go-sse/internal/store"
)

func main() {
	// Load environment variables from .env file if it exists
	godotenv.Load()

	// Get the port from the environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Get max request size from environment or use default (20MB)
	maxRequestSize := os.Getenv("MAX_REQUEST_SIZE_MB")
	maxRequestSizeMB := 20 // Default to 20MB
	if maxRequestSize != "" {
		if size, err := strconv.Atoi(maxRequestSize); err == nil && size > 0 {
			maxRequestSizeMB = size
		}
	}
	// Convert to bytes
	maxBodyBytes := int64(maxRequestSizeMB * 1024 * 1024)
	log.Printf("Maximum request body size: %dMB", maxRequestSizeMB)

	// Get the store type from environment or use default
	storeType := os.Getenv("STORE_TYPE")
	var storeTypeEnum store.StoreType
	switch storeType {
	case "mongo":
		storeTypeEnum = store.MongoStoreType
		log.Println("Using MongoDB store")
	default:
		storeTypeEnum = store.MemoryStore
		log.Println("Using in-memory store")
	}

	// Create the store
	kvStore, err := store.CreateStore(storeTypeEnum)
	if err != nil {
		log.Fatalf("Failed to create store: %v", err)
	}

	// Display store/database information at startup
	log.Println("Displaying initial store/database information")
	if err := kvStore.DisplayStoreInfo(); err != nil {
		log.Printf("Warning: Failed to display store information: %v", err)
	}

	// Create components
	sseServer := sse.NewServer(kvStore)
	apiHandler := api.NewHandler(kvStore, sseServer)
	router := api.SetupRouter(apiHandler)

	// Create HTTP server with middleware for large requests
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set body size limit based on route
			r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
			router.ServeHTTP(w, r)
		}),
		ReadTimeout:  120 * time.Second, // Increased for large uploads
		WriteTimeout: 120 * time.Second, // Increased for large responses
		IdleTimeout:  240 * time.Second, // Keep idle connections open longer
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Set up graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down server...")

	// Shutdown HTTP server
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second) // Increased shutdown timeout
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	// Shutdown SSE server
	sseServer.Shutdown()

	log.Println("Server stopped")
}

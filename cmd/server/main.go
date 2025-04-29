package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
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

	// Get the store type from environment or use default
	storeType := os.Getenv("STORE_TYPE")
	var storeTypeEnum store.StoreType
	switch storeType {
	case "mongo":
		storeTypeEnum = store.MongoStore
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

	// Create components
	sseServer := sse.NewServer(kvStore)
	apiHandler := api.NewHandler(kvStore, sseServer)
	router := api.SetupRouter(apiHandler)

	// Create HTTP server
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second, // Longer timeout for SSE connections
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	// Shutdown SSE server
	sseServer.Shutdown()

	log.Println("Server stopped")
}

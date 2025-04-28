package sse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/piske-alex/go-sse/internal/sse"
	"github.com/piske-alex/go-sse/internal/store"
)

func BenchmarkSSEServer_BroadcastEvent(b *testing.B) {
	// Number of concurrent clients
	clientCounts := []int{10, 100, 1000}

	for _, clientCount := range clientCounts {
		b.Run(fmt.Sprintf("clients-%d", clientCount), func(b *testing.B) {
			benchmarkBroadcastWithClients(b, clientCount)
		})
	}
}

func benchmarkBroadcastWithClients(b *testing.B, clientCount int) {
	// Create a store
	kvStore := store.NewStore()

	// Initialize with test data
	kvStore.Initialize(map[string]interface{}{
		"users": make([]interface{}, clientCount), // Create an array with clientCount elements
	})

	// Populate users
	users := make([]interface{}, clientCount)
	for i := 0; i < clientCount; i++ {
		users[i] = map[string]interface{}{
			"id":     i,
			"name":   fmt.Sprintf("User%d", i),
			"status": "online",
		}
	}
	kvStore.Set(".users", users)

	// Create SSE server
	sseServer := sse.NewServer(kvStore)

	// Create clients
	ctxs := make([]context.CancelFunc, clientCount)
	var wg sync.WaitGroup

	// Add clients in parallel for better performance
	wg.Add(clientCount)
	for i := 0; i < clientCount; i++ {
		go func(index int) {
			defer wg.Done()

			// Create a recorder for this client
			w := httptest.NewRecorder()

			// Create a request with cancelable context
			r := httptest.NewRequest("GET", "/events", nil)
			ctx, cancel := context.WithCancel(r.Context())
			ctxs[index] = cancel
			r = r.WithContext(ctx)

			// Add client with a filter for its own status
			filter := fmt.Sprintf(".users[%d].status", index)
			_, err := sseServer.AddClient(w, r, []string{filter})
			if err != nil {
				b.Fatalf("Failed to add client %d: %v", index, err)
			}
		}(i)
	}

	// Wait for all clients to be added
	wg.Wait()

	// Verify all clients were added successfully
	if count := sseServer.ClientCount(); count != clientCount {
		b.Fatalf("Expected %d clients, got %d", clientCount, count)
	}

	// Reset the timer to exclude setup time
	b.ResetTimer()

	// Run the actual benchmark
	for i := 0; i < b.N; i++ {
		// Update each user's status to trigger broadcasts
		for j := 0; j < clientCount; j++ {
			path := fmt.Sprintf(".users[%d].status", j)
			value := "away"
			sseServer.BroadcastEvent(path, value, "update")
		}
	}

	// Stop the timer
	b.StopTimer()

	// Clean up: cancel all contexts to remove clients
	for _, cancel := range ctxs {
		cancel()
	}

	// Wait a bit for cleanup
	time.Sleep(100 * time.Millisecond)

	// Shutdown the server
	sseServer.Shutdown()
}

package sse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/piske-alex/go-sse/internal/sse"
	"github.com/piske-alex/go-sse/internal/store"
)

func TestServer_ClientManagement(t *testing.T) {
	// Create a store
	kvStore := store.NewStore()

	// Create SSE server
	sseServer := sse.NewServer(kvStore)

	// Check initial client count
	initialCount := sseServer.ClientCount()
	if initialCount != 0 {
		t.Fatalf("Expected initial client count to be 0, got %d", initialCount)
	}

	// Create a recorder for testing
	w := httptest.NewRecorder()

	// Create a dummy request
	r := httptest.NewRequest("GET", "/events", nil)

	// Create a context that we can cancel
	ctx, cancel := context.WithCancel(r.Context())
	r = r.WithContext(ctx)

	// Add a client
	client, err := sseServer.AddClient(w, r, []string{".users[*].status"})
	if err != nil {
		t.Fatalf("Failed to add client: %v", err)
	}

	// Check client count after adding
	count := sseServer.ClientCount()
	if count != 1 {
		t.Fatalf("Expected client count to be 1, got %d", count)
	}

	// Force client disconnection by cancelling the context
	cancel()

	// Give some time for cleanup
	time.Sleep(100 * time.Millisecond)

	// Check client count after disconnection
	finalCount := sseServer.ClientCount()
	if finalCount != 0 {
		t.Fatalf("Expected final client count to be 0, got %d", finalCount)
	}
}

func TestServer_BroadcastEvent(t *testing.T) {
	// Create a store
	kvStore := store.NewStore()

	// Initialize with test data
	kvStore.Initialize(map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":     1,
				"name":   "Alice",
				"status": "online",
			},
		},
	})

	// Create SSE server
	sseServer := sse.NewServer(kvStore)

	// Create a recorder for testing
	w := httptest.NewRecorder()

	// Create a dummy request
	r := httptest.NewRequest("GET", "/events", nil)

	// Create a client with a specific filter
	client, err := sseServer.AddClient(w, r, []string{".users[*].status"})
	if err != nil {
		t.Fatalf("Failed to add client: %v", err)
	}

	// Broadcast an event that should match the filter
	sseServer.BroadcastEvent(".users[0].status", "away", "update")

	// Wait a bit for the event to be processed
	time.Sleep(100 * time.Millisecond)

	// Check the response
	resp := w.Result()
	if resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type to be text/event-stream, got %s", resp.Header.Get("Content-Type"))
	}

	// Clean up
	sseServer.RemoveClient(client.ID)

	// Verify client is removed
	count := sseServer.ClientCount()
	if count != 0 {
		t.Fatalf("Expected client count to be 0 after removal, got %d", count)
	}
}

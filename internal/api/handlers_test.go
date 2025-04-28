package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/piske-alex/go-sse/internal/api"
	"github.com/piske-alex/go-sse/internal/sse"
	"github.com/piske-alex/go-sse/internal/store"
)

func TestHandleStoreInitialize(t *testing.T) {
	// Create components
	kvStore := store.NewStore()
	sseServer := sse.NewServer(kvStore)
	apiHandler := api.NewHandler(kvStore, sseServer)

	// Test data
	testData := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":     1,
				"name":   "Alice",
				"status": "online",
			},
		},
	}

	// Convert to JSON
	jsonData, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// Create a request
	req := httptest.NewRequest("POST", "/store", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// Create a response recorder
	w := httptest.NewRecorder()

	// Call the handler
	apiHandler.HandleStoreInitialize(w, req)

	// Check the response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the store was initialized
	result, err := kvStore.Get("")
	if err != nil {
		t.Fatalf("Failed to get store data: %v", err)
	}

	// Check data structure
	resultData, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", result)
	}

	// Check for users key
	users, ok := resultData["users"]
	if !ok {
		t.Fatalf("users key not found in store")
	}

	// Check users array
	usersList, ok := users.([]interface{})
	if !ok {
		t.Fatalf("Expected users to be []interface{}, got %T", users)
	}

	// Check we have one user
	if len(usersList) != 1 {
		t.Fatalf("Expected 1 user, got %d", len(usersList))
	}
}

func TestHandleStoreUpdate(t *testing.T) {
	// Create components
	kvStore := store.NewStore()
	sseServer := sse.NewServer(kvStore)
	apiHandler := api.NewHandler(kvStore, sseServer)

	// Initialize the store
	kvStore.Initialize(map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":     1,
				"name":   "Alice",
				"status": "online",
			},
		},
	})

	// Test update value
	newStatus := "away"

	// Convert to JSON
	jsonData, err := json.Marshal(newStatus)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	// Create a request
	req := httptest.NewRequest("PATCH", "/store?path=.users[0].status", bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")

	// Add path parameter
	q := req.URL.Query()
	q.Add("path", ".users[0].status")
	req.URL.RawQuery = q.Encode()

	// Create a response recorder
	w := httptest.NewRecorder()

	// Call the handler
	apiHandler.HandleStoreUpdate(w, req)

	// Check the response
	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, resp.StatusCode)
	}

	// Verify the store was updated
	result, err := kvStore.Get(".users[0].status")
	if err != nil {
		t.Fatalf("Failed to get updated value: %v", err)
	}

	// Check updated value
	if result != "away" {
		t.Errorf("Expected status to be updated to 'away', got %v", result)
	}
}

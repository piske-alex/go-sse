package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/piske-alex/go-sse/internal/sse"
	"github.com/piske-alex/go-sse/internal/store"
)

// Handler manages the HTTP API handlers
type Handler struct {
	Store     interface{} // Generalized to work with any store type
	SSEServer *sse.Server
}

// NewHandler creates a new API handler
func NewHandler(store interface{}, sseServer *sse.Server) *Handler {
	return &Handler{
		Store:     store,
		SSEServer: sseServer,
	}
}

// HandleEvents handles SSE connections
func (h *Handler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse filter query parameter
	filters := []string{}
	filterParam := r.URL.Query().Get("filter")
	if filterParam != "" {
		filters = strings.Split(filterParam, ",")
	}

	// Add client to SSE server
	_, err := h.SSEServer.AddClient(w, r, filters)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Keep the connection open until client disconnects
	<-r.Context().Done()
}

// HandleStoreInitialize handles store initialization
func (h *Handler) HandleStoreInitialize(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Handle based on store type
	switch s := h.Store.(type) {
	case *store.Store: // In-memory store
		err = s.InitializeFromJSON(body)
	
	case *store.MongoStore: // MongoDB store
		err = s.InitializeFromJSON(body)
	
	default:
		http.Error(w, "Unsupported store type", http.StatusInternalServerError)
		return
	}

	if err != nil {
		http.Error(w, "Invalid JSON data: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Broadcast store initialization event
	h.SSEServer.BroadcastEvent(".", nil, "init")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleStoreUpdate handles store updates
func (h *Handler) HandleStoreUpdate(w http.ResponseWriter, r *http.Request) {
	// Only allow PATCH requests
	if r.Method != http.MethodPatch {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get path from query parameter
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Handle based on store type
	switch s := h.Store.(type) {
	case *store.Store: // In-memory store
		err = s.SetFromJSON(path, body)
	
	case *store.MongoStore: // MongoDB store
		err = s.SetFromJSON(path, body)
		
	default:
		http.Error(w, "Unsupported store type", http.StatusInternalServerError)
		return
	}

	if err != nil {
		http.Error(w, "Error updating store: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Get the updated value for broadcasting
	var updatedValue interface{}

	switch s := h.Store.(type) {
	case *store.Store: // In-memory store
		updatedValue, err = s.Get(path)
	
	case *store.MongoStore: // MongoDB store
		updatedValue, err = s.Get(path)
		
	default:
		http.Error(w, "Unsupported store type", http.StatusInternalServerError)
		return
	}

	if err != nil {
		// If we can't get the value, still return success but don't broadcast
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
		return
	}

	// Broadcast update event
	h.SSEServer.BroadcastEvent(path, updatedValue, "update")

	// Return success response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// HandleStoreQuery handles store queries
func (h *Handler) HandleStoreQuery(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get path from query parameter
	path := r.URL.Query().Get("path")

	// Query the store
	var result interface{}
	var err error

	if strings.Contains(path, "*") {
		// Path contains wildcards, use FindMatches
		switch s := h.Store.(type) {
		case *store.Store: // In-memory store
			result, err = s.FindMatches(path)
		
		case *store.MongoStore: // MongoDB store
			result, err = s.FindMatches(path)
			
		default:
			http.Error(w, "Unsupported store type", http.StatusInternalServerError)
			return
		}
	} else {
		// Simple path, use Get
		switch s := h.Store.(type) {
		case *store.Store: // In-memory store
			result, err = s.Get(path)
		
		case *store.MongoStore: // MongoDB store
			result, err = s.Get(path)
			
		default:
			http.Error(w, "Unsupported store type", http.StatusInternalServerError)
			return
		}
	}

	if err != nil {
		http.Error(w, "Error querying store: "+err.Error(), http.StatusNotFound)
		return
	}

	// Return result as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// HandleMetrics returns server metrics
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get metrics
	metrics := map[string]interface{}{
		"clients": h.SSEServer.ClientCount(),
		"time":    time.Now().UnixNano() / int64(time.Millisecond),
	}

	// Add store type
	switch h.Store.(type) {
	case *store.Store:
		metrics["store_type"] = "memory"
	case *store.MongoStore:
		metrics["store_type"] = "mongodb"
	default:
		metrics["store_type"] = "unknown"
	}

	// Return metrics as JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

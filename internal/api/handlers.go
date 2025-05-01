package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/piske-alex/go-sse/internal/sse"
	"github.com/piske-alex/go-sse/internal/store"
)

// Handler manages the HTTP API handlers
type Handler struct {
	Store     store.Store // Use store.Store interface instead of interface{}
	SSEServer *sse.Server
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a success response
type SuccessResponse struct {
	Status  string      `json:"status"`
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// NewHandler creates a new API handler
func NewHandler(dataStore store.Store, sseServer *sse.Server) *Handler {
	return &Handler{
		Store:     dataStore,
		SSEServer: sseServer,
	}
}

// sendJSONError sends a JSON error response
func sendJSONError(w http.ResponseWriter, statusCode int, errorType string, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	resp := ErrorResponse{
		Error:   errorType,
		Code:    statusCode,
		Message: message,
	}

	json.NewEncoder(w).Encode(resp)
}

// sendJSONSuccess sends a JSON success response
func sendJSONSuccess(w http.ResponseWriter, data interface{}, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	resp := SuccessResponse{
		Status:  "success",
		Data:    data,
		Message: message,
	}

	json.NewEncoder(w).Encode(resp)
}

// HandleEvents handles SSE connections
func (h *Handler) HandleEvents(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		sendJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET requests are allowed for SSE connections")
		return
	}

	// Parse filter query parameter
	filters := []string{}
	filterParam := r.URL.Query().Get("filter")
	if filterParam != "" {
		filters = strings.Split(filterParam, ",")
	}

	// Add client to SSE server
	client, err := h.SSEServer.AddClient(w, r, filters)
	if err != nil {
		log.Printf("Error adding SSE client: %v", err)
		sendJSONError(w, http.StatusInternalServerError, "sse_connection_failed", fmt.Sprintf("Failed to establish SSE connection: %v", err))
		return
	}

	// Log client connection
	log.Printf("SSE client connected: %s with filters: %v", client.ID, filters)

	// Keep the connection open until client disconnects
	<-r.Context().Done()
	log.Printf("SSE client disconnected: %s", client.ID)
}

// HandleStoreInitialize handles store initialization
func (h *Handler) HandleStoreInitialize(w http.ResponseWriter, r *http.Request) {
	// Only allow POST requests
	if r.Method != http.MethodPost {
		sendJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only POST requests are allowed for store initialization")
		return
	}

	// Validate content type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		sendJSONError(w, http.StatusUnsupportedMediaType, "invalid_content_type", "Content-Type must be application/json")
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		sendJSONError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Error reading request body: %v", err))
		return
	}
	defer r.Body.Close()

	// Validate JSON format first
	var jsonTest interface{}
	if err := json.Unmarshal(body, &jsonTest); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid_json", fmt.Sprintf("Invalid JSON format: %v", err))
		return
	}

	// Log operation
	log.Printf("Initializing store with %d bytes of JSON data", len(body))

	// Use the Store interface directly, no need for type switch
	err = h.Store.InitializeFromJSON(body)

	if err != nil {
		log.Printf("Error initializing store: %v", err)
		sendJSONError(w, http.StatusBadRequest, "initialization_failed", fmt.Sprintf("Failed to initialize store: %v", err))
		return
	}

	// Broadcast store initialization event
	h.SSEServer.BroadcastEvent(".", nil, "init")

	// Return success response with information about the operation
	sendJSONSuccess(w, map[string]interface{}{
		"size_bytes": len(body),
		"timestamp":  time.Now().Unix(),
	}, "Store initialized successfully")
}

// HandleStoreUpdate handles store updates
func (h *Handler) HandleStoreUpdate(w http.ResponseWriter, r *http.Request) {
	// Only allow PATCH requests
	if r.Method != http.MethodPatch {
		sendJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only PATCH requests are allowed for store updates")
		return
	}

	// Validate content type
	contentType := r.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		sendJSONError(w, http.StatusUnsupportedMediaType, "invalid_content_type", "Content-Type must be application/json")
		return
	}

	// Get path from query parameter
	path := r.URL.Query().Get("path")
	if path == "" {
		sendJSONError(w, http.StatusBadRequest, "missing_parameter", "Missing path parameter")
		return
	}

	// Read request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		sendJSONError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("Error reading request body: %v", err))
		return
	}
	defer r.Body.Close()

	// Validate JSON format first
	var jsonTest interface{}
	if err := json.Unmarshal(body, &jsonTest); err != nil {
		sendJSONError(w, http.StatusBadRequest, "invalid_json", fmt.Sprintf("Invalid JSON format: %v", err))
		return
	}

	// Log operation
	log.Printf("Updating store at path '%s' with %d bytes of JSON data", path, len(body))

	// Use the Store interface directly
	err = h.Store.SetFromJSON(path, body)

	if err != nil {
		log.Printf("Error updating store: %v", err)
		sendJSONError(w, http.StatusBadRequest, "update_failed", fmt.Sprintf("Failed to update store: %v", err))
		return
	}

	// Get the updated value
	value, err := h.Store.Get(path)
	if err != nil {
		log.Printf("Error getting updated value: %v", err)
	} else {
		// Broadcast update event if value was retrieved successfully
		h.SSEServer.BroadcastEvent(path, value, "update")
	}

	// Return success response
	sendJSONSuccess(w, map[string]interface{}{
		"path":       path,
		"size_bytes": len(body),
		"timestamp":  time.Now().Unix(),
	}, "Store updated successfully")
}

// HandleStoreQuery handles store queries
func (h *Handler) HandleStoreQuery(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		sendJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET requests are allowed for store queries")
		return
	}

	// Get path from query parameter
	path := r.URL.Query().Get("path")
	if path == "" {
		sendJSONError(w, http.StatusBadRequest, "missing_parameter", "Missing path parameter")
		return
	}

	// Check if this is a pattern match query
	isPattern := false
	patternParam := r.URL.Query().Get("pattern")
	if patternParam == "true" {
		isPattern = true
	}

	// Log operation
	log.Printf("Querying store at path '%s' (pattern: %v)", path, isPattern)

	// Different handling for pattern matches vs direct query
	var (
		result interface{}
		err    error
	)

	if isPattern {
		// Pattern match query, use FindMatches
		result, err = h.Store.FindMatches(path)
	} else {
		// Simple path, use Get
		result, err = h.Store.Get(path)
	}

	if err != nil {
		log.Printf("Error querying store: %v", err)
		sendJSONError(w, http.StatusNotFound, "query_failed", fmt.Sprintf("Failed to query store at path '%s': %v", path, err))
		return
	}

	// Return result directly as JSON
	w.Header().Set("Content-Type", "application/json")
	
	// Create a more efficient encoder for large JSON responses
	encoder := json.NewEncoder(w)
	encoder.SetEscapeHTML(false) // Improve performance by not escaping HTML
	encoder.SetIndent("", "")   // No indentation for smaller payload
	
	if err := encoder.Encode(result); err != nil {
		log.Printf("Error encoding JSON response: %v", err)
		sendJSONError(w, http.StatusInternalServerError, "encoding_error", "Failed to encode response")
	}
}

// HandleHealth returns a simple health check response
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		sendJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET requests are allowed for health check")
		return
	}

	// Return success response
	sendJSONSuccess(w, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().Unix(),
	}, "Service is healthy")
}

// HandleMetrics returns server metrics
func (h *Handler) HandleMetrics(w http.ResponseWriter, r *http.Request) {
	// Only allow GET requests
	if r.Method != http.MethodGet {
		sendJSONError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Only GET requests are allowed for metrics")
		return
	}

	// Get metrics
	metrics := map[string]interface{}{
		"clients":    h.SSEServer.ClientCount(),
		"time":       time.Now().Unix(),
		"uptime":     time.Now().Unix(), // This should be replaced with actual uptime
		"store_type": "unknown",
	}

	// Add store type by checking concrete type
	switch h.Store.(type) {
	case *store.KVStore:
		metrics["store_type"] = "memory"
	case *store.MongoStore:
		metrics["store_type"] = "mongodb"
	}

	// Return metrics as JSON
	sendJSONSuccess(w, metrics, "Server metrics")
}

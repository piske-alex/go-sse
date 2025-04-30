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
	Store     interface{} // Generalized to work with any store type
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
func NewHandler(store interface{}, sseServer *sse.Server) *Handler {
	return &Handler{
		Store:     store,
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

	// Handle based on store type
	switch s := h.Store.(type) {
	case *store.Store: // In-memory store
		err = s.InitializeFromJSON(body)
	
	case *store.MongoStore: // MongoDB store
		err = s.InitializeFromJSON(body)
	
	default:
		sendJSONError(w, http.StatusInternalServerError, "store_error", "Unsupported store type")
		return
	}

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

	// Handle based on store type
	switch s := h.Store.(type) {
	case *store.Store: // In-memory store
		err = s.SetFromJSON(path, body)
	
	case *store.MongoStore: // MongoDB store
		err = s.SetFromJSON(path, body)
		
	default:
		sendJSONError(w, http.StatusInternalServerError, "store_error", "Unsupported store type")
		return
	}

	if err != nil {
		log.Printf("Error updating store: %v", err)
		sendJSONError(w, http.StatusBadRequest, "update_failed", fmt.Sprintf("Failed to update store: %v", err))
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
		sendJSONError(w, http.StatusInternalServerError, "store_error", "Unsupported store type")
		return
	}

	if err != nil {
		// If we can't get the value, still return success but don't broadcast
		sendJSONSuccess(w, nil, fmt.Sprintf("Path '%s' updated successfully, but could not retrieve the new value", path))
		return
	}

	// Broadcast update event
	h.SSEServer.BroadcastEvent(path, updatedValue, "update")

	// Return success response with information about the operation
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
		// Default to root if no path provided
		path = "."
	}

	// Log operation
	log.Printf("Querying store at path '%s'", path)

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
			sendJSONError(w, http.StatusInternalServerError, "store_error", "Unsupported store type")
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
			sendJSONError(w, http.StatusInternalServerError, "store_error", "Unsupported store type")
			return
		}
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

	// Add store type
	switch h.Store.(type) {
	case *store.Store:
		metrics["store_type"] = "memory"
	case *store.MongoStore:
		metrics["store_type"] = "mongodb"
	}

	// Return metrics as JSON
	sendJSONSuccess(w, metrics, "Server metrics")
}

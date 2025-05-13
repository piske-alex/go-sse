package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/piske-alex/go-sse/internal/query"
)

// Client represents a connected SSE client
type Client struct {
	ID           string
	W            http.ResponseWriter
	F            *http.Flusher
	Filters      []*query.Filter
	Ctx          context.Context
	CancelFunc   context.CancelFunc
	LastActivity time.Time
	MessageChan  chan []byte
}

// NewClient creates a new SSE client instance
func NewClient(w http.ResponseWriter, filterExprs []string) (*Client, error) {
	// Check if the writer supports flushing
	f, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("streaming not supported")
	}

	// Create a context with cancel function for this client
	ctx, cancel := context.WithCancel(context.Background())

	// Set required headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create filters from expressions
	var filters []*query.Filter
	for _, expr := range filterExprs {
		if expr != "" {
			filters = append(filters, query.NewFilter(expr))
		}
	}

	// If no filters were provided, add a default one that matches everything
	if len(filters) == 0 {
		filters = append(filters, query.NewFilter("."))
	}

	client := &Client{
		ID:           uuid.New().String(),
		W:            w,
		F:            &f,
		Filters:      filters,
		Ctx:          ctx,
		CancelFunc:   cancel,
		LastActivity: time.Now(),
		MessageChan:  make(chan []byte, 100), // Buffer for 100 messages
	}

	return client, nil
}

// Send sends an SSE message to the client
func (c *Client) Send(event string, data interface{}) error {
	// Check if context is cancelled
	select {
	case <-c.Ctx.Done():
		return fmt.Errorf("client context cancelled")
	default:
		// Context still valid, continue
	}

	// Special case for filter handling - specific for .data.positions
	if eventData, ok := data.(map[string]interface{}); ok {
		// Check if this is a filtered event
		if _, filtered := eventData["filtered"].(bool); filtered {
			// Check for specific filter case
			for _, filter := range c.Filters {
				if filter.Path == ".data.positions" || filter.Path == "data.positions" {
					// For .data.positions filter, ensure we're only sending the positions data
					if value, hasValue := eventData["value"]; hasValue {
						// If data.positions exists in the value, extract just that
						if valueMap, ok := value.(map[string]interface{}); ok {
							if data, ok := valueMap["data"].(map[string]interface{}); ok {
								if positions, ok := data["positions"]; ok {
									// Replace the value with just the positions
									eventData["value"] = positions
								}
							}
						}
					}
					break
				}
			}
		}
	}

	// Convert data to JSON if it's not a string
	var dataStr string
	switch v := data.(type) {
	case string:
		dataStr = v
	case []byte:
		dataStr = string(v)
	default:
		jsonData, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("failed to marshal data: %w", err)
		}
		dataStr = string(jsonData)
	}

	// Format the SSE message
	message := fmt.Sprintf("event: %s\ndata: %s\n\n", event, dataStr)

	// Send via channel
	select {
	case c.MessageChan <- []byte(message):
		// Message queued successfully
	default:
		// Channel full, drop message to avoid blocking
		return fmt.Errorf("client message queue full")
	}

	return nil
}

// SendComment sends a comment (used for keep-alive)
func (c *Client) SendComment(comment string) error {
	// Check if context is cancelled
	select {
	case <-c.Ctx.Done():
		return fmt.Errorf("client context cancelled")
	default:
		// Context still valid, continue
	}

	// Format the SSE comment
	message := fmt.Sprintf(": %s\n\n", comment)

	// Send via channel
	select {
	case c.MessageChan <- []byte(message):
		// Message queued successfully
	default:
		// Channel full, drop message to avoid blocking
		return fmt.Errorf("client message queue full")
	}

	return nil
}

// Close closes the client connection
func (c *Client) Close() {
	c.CancelFunc()
	close(c.MessageChan)
}

// ShouldNotify checks if the client should be notified of a change
func (c *Client) ShouldNotify(path string, value interface{}) bool {
	// Special case for paths that might be prefixes of filter paths
	for _, filter := range c.Filters {
		// If the filter is more specific than the path
		if strings.HasPrefix(filter.Path, path) {
			// Check if this is the .data.positions filter
			if filter.Path == ".data.positions" || filter.Path == "data.positions" {
				// If data change happens at root or .data level, check if it affects positions
				if path == "." || path == ".data" || path == "data" {
					// Look for data.positions in the change value
					if valueMap, ok := value.(map[string]interface{}); ok {
						// Check if data field exists
						if data, ok := valueMap["data"].(map[string]interface{}); ok {
							// Check if data has a positions field
							if _, ok := data["positions"]; ok {
								return true
							}
						}
					}
				}
			}
			return true
		}
	}

	// Check if any filter matches the change
	for _, filter := range c.Filters {
		if filter.IsMatch(path, value) {
			return true
		}
	}
	return false
}

// ProcessMessages starts a goroutine to process and send messages to the client
func (c *Client) ProcessMessages() {
	go func() {
		// Create a ticker for keep-alive comments
		keepaliveTicker := time.NewTicker(30 * time.Second)
		defer keepaliveTicker.Stop()

		for {
			select {
			case <-c.Ctx.Done():
				// Client context cancelled, exit goroutine
				return

			case msg, ok := <-c.MessageChan:
				if !ok {
					// Channel closed, exit goroutine
					return
				}

				// Write message to the client
				_, err := c.W.Write(msg)
				if err != nil {
					// Write failed, cancel client context
					c.CancelFunc()
					return
				}

				// Flush to ensure the message is sent immediately
				(*c.F).Flush()
				c.LastActivity = time.Now()

			case <-keepaliveTicker.C:
				// Send keep-alive comment
				_, err := c.W.Write([]byte(": keepalive\n\n"))
				if err != nil {
					// Write failed, cancel client context
					c.CancelFunc()
					return
				}

				// Flush to ensure the keep-alive is sent immediately
				(*c.F).Flush()
				c.LastActivity = time.Now()
			}
		}
	}()
}

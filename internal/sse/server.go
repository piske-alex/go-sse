package sse

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/piske-alex/go-sse/internal/store"
)

// Server manages SSE client connections and broadcasting
type Server struct {
	store          store.Store  // Use the Store interface instead of interface{}
	clients        map[string]*Client
	clientsMutex   sync.RWMutex
	maxClients     int
	cleanupTicker  *time.Ticker
	cleanupContext context.Context
	cleanupCancel  context.CancelFunc
}

// NewServer creates a new SSE server instance
func NewServer(dataStore store.Store) *Server {
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())

	s := &Server{
		store:          dataStore,
		clients:        make(map[string]*Client),
		clientsMutex:   sync.RWMutex{},
		maxClients:     10000, // Maximum number of clients
		cleanupTicker:  time.NewTicker(5 * time.Minute),
		cleanupContext: cleanupCtx,
		cleanupCancel:  cleanupCancel,
	}

	// MongoDB specific operations need to be handled differently since MongoStore is custom type
	// We can look for a specific interface method only available on MongoStore
	if mongoStore, ok := dataStore.(*store.MongoStore); ok {
		// If this is a MongoStore, register change listener
		if setListener, ok := interface{}(mongoStore).(interface{ SetChangeListener(func(string, interface{})) }); ok {
			setListener.SetChangeListener(func(path string, value interface{}) {
				s.BroadcastEvent(path, value, "update")
			})
		}
	}

	// Start the cleanup goroutine
	go s.startCleanup()

	return s
}

// AddClient adds a new client connection
func (s *Server) AddClient(w http.ResponseWriter, r *http.Request, filterExprs []string, sendInitialData bool) (*Client, error) {
	// Check if we've reached max clients
	s.clientsMutex.RLock()
	if len(s.clients) >= s.maxClients {
		s.clientsMutex.RUnlock()
		return nil, http.ErrHandlerTimeout
	}
	s.clientsMutex.RUnlock()

	// Create a new client
	client, err := NewClient(w, filterExprs)
	if err != nil {
		return nil, err
	}

	// Add client to the server
	s.clientsMutex.Lock()
	s.clients[client.ID] = client
	s.clientsMutex.Unlock()

	// Start processing client messages
	client.ProcessMessages()

	// Set up a goroutine to remove client when connection closes
	go func() {
		<-r.Context().Done()
		s.RemoveClient(client.ID)
	}()

	// Send initial connection event
	client.Send("connected", map[string]string{"id": client.ID})

	// If sendInitialData is false, skip sending the initial data
	if !sendInitialData {
		log.Printf("Skipping initial data for client %s as requested", client.ID)
		return client, nil
	}

	// Small delay to ensure connection event is processed first
	time.Sleep(50 * time.Millisecond)

	// Send initial store data to the client
	// Try to respect filters if they exist
	if len(client.Filters) > 0 {
		// Get the root data first
		rootData, err := s.store.Get(".")
		if err != nil {
			log.Printf("Error fetching initial data for client %s: %v", client.ID, err)
		} else if rootData != nil {
			// Create a map to deduplicate filtered data
			sent := make(map[string]bool)
			
			// For each filter, try to find matching data
			for _, filter := range client.Filters {
				// Simple case: if filter is "." or empty, send all data
				if filter.Path == "." || filter.Path == "" {
					eventData := map[string]interface{}{
						"path":  ".",
						"value": rootData,
						"time":  time.Now().UnixNano() / int64(time.Millisecond),
					}
					client.Send("initial_data", eventData)
					sent["."] = true
					continue
				}
				
				// Try to get data for the specific filter path
				data, err := s.store.Get(filter.Path)
				if err != nil {
					// If direct path doesn't work, try pattern matching
					matches, err := s.store.FindMatches(filter.Path) 
					if err == nil && len(matches) > 0 {
						// Send each match that hasn't been sent yet
						for _, match := range matches {
							if !sent[match.Path] {
								eventData := map[string]interface{}{
									"path":  match.Path,
									"value": match.Value,
									"time":  time.Now().UnixNano() / int64(time.Millisecond),
								}
								client.Send("initial_data", eventData)
								sent[match.Path] = true
							}
						}
					}
				} else if data != nil && !sent[filter.Path] {
					// Send the data for this filter
					eventData := map[string]interface{}{
						"path":  filter.Path,
						"value": data,
						"time":  time.Now().UnixNano() / int64(time.Millisecond),
					}
					client.Send("initial_data", eventData)
					sent[filter.Path] = true
				}
			}
		}
	} else {
		// No specific filters, just send the root data
		initialData, err := s.store.Get(".")
		if err == nil && initialData != nil {
			eventData := map[string]interface{}{
				"path":  ".",
				"value": initialData,
				"time":  time.Now().UnixNano() / int64(time.Millisecond),
			}
			client.Send("initial_data", eventData)
		} else {
			// Log error but don't fail the connection
			log.Printf("Error fetching initial data for client %s: %v", client.ID, err)
		}
	}

	return client, nil
}

// RemoveClient removes a client connection
func (s *Server) RemoveClient(clientID string) {
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	// Get the client
	client, exists := s.clients[clientID]
	if !exists {
		return
	}

	// Close the client
	client.Close()

	// Remove from clients map
	delete(s.clients, clientID)
}

// BroadcastEvent sends an event to all matching clients
func (s *Server) BroadcastEvent(path string, value interface{}, eventType string) {
	// Create a list of clients to notify
	s.clientsMutex.RLock()
	var clientsToNotify []*Client
	for _, client := range s.clients {
		if client.ShouldNotify(path, value) {
			clientsToNotify = append(clientsToNotify, client)
		}
	}
	s.clientsMutex.RUnlock()

	// Create the event payload
	eventData := map[string]interface{}{
		"path":  path,
		"value": value,
		"time":  time.Now().UnixNano() / int64(time.Millisecond),
	}

	// Send to all matching clients
	for _, client := range clientsToNotify {
		client.Send(eventType, eventData)
	}
}

// Shutdown gracefully shuts down the SSE server
func (s *Server) Shutdown() {
	// Stop the cleanup goroutine
	s.cleanupCancel()
	s.cleanupTicker.Stop()

	// Close all client connections
	s.clientsMutex.Lock()
	defer s.clientsMutex.Unlock()

	for id, client := range s.clients {
		client.Close()
		delete(s.clients, id)
	}

	// MongoDB specific shutdown
	if mongoStore, ok := s.store.(*store.MongoStore); ok {
		// If this is a MongoStore, disconnect from MongoDB
		if disconnect, ok := interface{}(mongoStore).(interface{ Disconnect() error }); ok {
			disconnect.Disconnect()
		}
	}
}

// ClientCount returns the number of connected clients
func (s *Server) ClientCount() int {
	s.clientsMutex.RLock()
	defer s.clientsMutex.RUnlock()
	return len(s.clients)
}

// startCleanup regularly checks for inactive clients and removes them
func (s *Server) startCleanup() {
	for {
		select {
		case <-s.cleanupContext.Done():
			return
		case <-s.cleanupTicker.C:
			s.cleanupInactiveClients()
		}
	}
}

// cleanupInactiveClients removes clients that haven't had activity in a while
func (s *Server) cleanupInactiveClients() {
	// Set the inactivity threshold (2 minutes)
	inactivityThreshold := time.Now().Add(-2 * time.Minute)

	// Collect inactive client IDs
	s.clientsMutex.RLock()
	var inactiveClients []string
	for id, client := range s.clients {
		if client.LastActivity.Before(inactivityThreshold) {
			inactiveClients = append(inactiveClients, id)
		}
	}
	s.clientsMutex.RUnlock()

	// Remove inactive clients
	for _, id := range inactiveClients {
		s.RemoveClient(id)
	}
}

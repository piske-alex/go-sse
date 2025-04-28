package sse

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/piske-alex/go-sse/internal/store"
)

// Server manages SSE client connections and broadcasting
type Server struct {
	store          *store.Store
	clients        map[string]*Client
	clientsMutex   sync.RWMutex
	maxClients     int
	cleanupTicker  *time.Ticker
	cleanupContext context.Context
	cleanupCancel  context.CancelFunc
}

// NewServer creates a new SSE server instance
func NewServer(store *store.Store) *Server {
	cleanupCtx, cleanupCancel := context.WithCancel(context.Background())

	s := &Server{
		store:          store,
		clients:        make(map[string]*Client),
		clientsMutex:   sync.RWMutex{},
		maxClients:     10000, // Maximum number of clients
		cleanupTicker:  time.NewTicker(5 * time.Minute),
		cleanupContext: cleanupCtx,
		cleanupCancel:  cleanupCancel,
	}

	// Start the cleanup goroutine
	go s.startCleanup()

	return s
}

// AddClient adds a new client connection
func (s *Server) AddClient(w http.ResponseWriter, r *http.Request, filterExprs []string) (*Client, error) {
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

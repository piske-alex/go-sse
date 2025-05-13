package sse

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/piske-alex/go-sse/internal/store"
	"github.com/piske-alex/go-sse/internal/query"
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
	if mongoStore, ok := dataStore.(*store.MongoStore); ok {
		// If this is a MongoStore, register change listener
		// All MongoStore instances have the SetChangeListener method
		mongoStore.SetChangeListener(func(path string, value interface{}) {
			// We need to determine if this is an update or a delete based on the value
			eventType := "update"
			if value == nil {
				eventType = "delete"
			}
			
			s.BroadcastEvent(path, value, eventType)
		})
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
		log.Printf("Fetching initial data with %d filters for client %s", len(client.Filters), client.ID)
		rootData, err := s.store.Get(".")
		if err != nil {
			log.Printf("Error fetching initial data for client %s: %v", client.ID, err)
		} else if rootData != nil {
			// Create a map to deduplicate filtered data
			sent := make(map[string]bool)
			
			// For each filter, try to find matching data
			for _, filter := range client.Filters {
				log.Printf("Processing filter '%s' for client %s", filter.Expression, client.ID)
				
				// Check if this filter has key-value conditions
				hasConditions := len(filter.Conditions) > 0
				if hasConditions {
					log.Printf("Filter has %d key-value conditions: %+v", len(filter.Conditions), filter.Conditions)
				}
				
				// Simple case: if filter is "." or empty, send all data
				if filter.Path == "." || filter.Path == "" {
					log.Printf("Filter is root path, sending all data to client %s", client.ID)
					
					// If we have conditions, we need to filter the root data
					if hasConditions && rootData != nil {
						// Try to filter the root data based on conditions
						if rootMap, ok := rootData.(map[string]interface{}); ok {
							// Check for data field that might contain filterable objects
							if dataField, hasData := rootMap["data"].(map[string]interface{}); hasData {
								// Process each data field
								for field, value := range dataField {
									// For array data, apply key-value filtering
									if arrayData, isArray := value.([]interface{}); isArray && len(arrayData) > 0 {
										log.Printf("Applying key-value filtering to initial data field: %s", field)
										
										// Use the common filtering function to ensure consistent behavior
										if filteredValue, success := applyKeyValueFilters(arrayData, filter.Conditions); success {
											log.Printf("Key-value filtering reduced array from %d items to %d items", 
												len(arrayData), len(filteredValue.([]interface{})))
											
											// If we have filtered results, send them
											eventData := map[string]interface{}{
												"path":              ".data." + field,
												"value":             filteredValue,
												"time":              time.Now().UnixNano() / int64(time.Millisecond),
												"filtered":          true,
												"filtered_field":    field,
												"key_value_filtered": true,
											}
											client.Send("initial_data", eventData)
											sent[".data."+field] = true
										}
									}
								}
							}
						}
					} else {
						// No conditions, send all data
						eventData := map[string]interface{}{
							"path":  ".",
							"value": rootData,
							"time":  time.Now().UnixNano() / int64(time.Millisecond),
						}
						client.Send("initial_data", eventData)
						sent["."] = true
					}
					continue
				}
				
				// Try to get data for the specific filter path
				log.Printf("Attempting direct path lookup for '%s' for client %s", filter.Path, client.ID)
				data, err := s.store.Get(filter.Path)
				if err != nil {
					// If direct path doesn't work, try pattern matching
					log.Printf("Direct path lookup failed, trying pattern matching for '%s' for client %s", filter.Path, client.ID)
					matches, err := s.store.FindMatches(filter.Path) 
					if err == nil && len(matches) > 0 {
						log.Printf("Found %d pattern matches for '%s' for client %s", len(matches), filter.Path, client.ID)
						// Send each match that hasn't been sent yet
						for _, match := range matches {
							if !sent[match.Path] {
								// Apply key-value filtering if needed
								valueToSend := match.Value
								if hasConditions {
									// Use the common filtering function to ensure consistent behavior
									if filteredValue, success := applyKeyValueFilters(valueToSend, filter.Conditions); success {
										log.Printf("Applied key-value filtering to initial data match %s", match.Path)
										valueToSend = filteredValue
									}
								}
								
								// Check if after filtering we have valid data to send
								// For arrays, check if there are any items left
								shouldSend := true
								if array, isArray := valueToSend.([]interface{}); isArray && len(array) == 0 {
									log.Printf("Skipping empty array result after filtering for %s", match.Path)
									shouldSend = false
								}
								
								if shouldSend {
									// Create and send the event
									eventData := map[string]interface{}{
										"path":              match.Path,
										"value":             valueToSend,
										"time":              time.Now().UnixNano() / int64(time.Millisecond),
										"filtered":          true,
										"key_value_filtered": hasConditions,
									}
									client.Send("initial_data", eventData)
									sent[match.Path] = true
									log.Printf("Sent filtered initial data for %s to client %s", match.Path, client.ID)
								}
							}
						}
					} else {
						log.Printf("No pattern matches found for '%s' for client %s: %v", filter.Path, client.ID, err)
					}
				} else if data != nil {
					if !sent[filter.Path] {
						// Apply key-value filtering if needed
						valueToSend := data
						if hasConditions {
							// Use the common filtering function to ensure consistent behavior
							if filteredValue, success := applyKeyValueFilters(valueToSend, filter.Conditions); success {
								log.Printf("Applied key-value filtering to initial data for path %s", filter.Path)
								valueToSend = filteredValue
							}
						}
						
						// Check if after filtering we have valid data to send
						// For arrays, check if there are any items left
						shouldSend := true
						if array, isArray := valueToSend.([]interface{}); isArray && len(array) == 0 {
							log.Printf("Skipping empty array result after filtering for %s", filter.Path)
							shouldSend = false
						}
						
						if shouldSend {
							// Create and send the event
							eventData := map[string]interface{}{
								"path":              filter.Path,
								"value":             valueToSend,
								"time":              time.Now().UnixNano() / int64(time.Millisecond),
								"filtered":          true,
								"key_value_filtered": hasConditions,
							}
							client.Send("initial_data", eventData)
							sent[filter.Path] = true
							log.Printf("Sent filtered initial data for %s to client %s", filter.Path, client.ID)
						}
					}
				} else {
					log.Printf("No data found for filter path '%s' for client %s", filter.Path, client.ID)
				}
			}
		}
	} else {
		// No specific filters, just send the root data
		log.Printf("No filters specified, sending root data to client %s", client.ID)
		initialData, err := s.store.Get(".")
		if err == nil && initialData != nil {
			eventData := map[string]interface{}{
				"path":  ".",
				"value": initialData,
				"time":  time.Now().UnixNano() / int64(time.Millisecond),
			}
			client.Send("initial_data", eventData)
			log.Printf("Successfully sent root data to client %s", client.ID)
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
	// Log the original event data
	log.Printf("DEBUG: BroadcastEvent called with path: %s, eventType: %s", path, eventType)
	
	// Check if the path contains key-value conditions
	if strings.Contains(path, "[") && strings.Contains(path, "=") && strings.Contains(path, "]") {
		log.Printf("DEBUG: Path contains key-value conditions: %s", path)
	}
	
	// Create a list of clients to notify
	s.clientsMutex.RLock()
	var clientsToNotify []*Client
	for _, client := range s.clients {
		if client.ShouldNotify(path, value) {
			clientsToNotify = append(clientsToNotify, client)
		}
	}
	s.clientsMutex.RUnlock()
	
	log.Printf("DEBUG: Found %d clients to notify", len(clientsToNotify))

	// Create the event payload
	eventData := map[string]interface{}{
		"path":  path,
		"value": value,
		"time":  time.Now().UnixNano() / int64(time.Millisecond),
	}

	// Send to all matching clients
	for _, client := range clientsToNotify {
		// Log filters for this client
		filterPaths := make([]string, 0, len(client.Filters))
		for _, f := range client.Filters {
			filterPaths = append(filterPaths, f.Expression) // Use Expression instead of Path to include conditions
		}
		log.Printf("DEBUG: Client %s has filters: %v", client.ID, filterPaths)
		
		// For each client, check if we need to apply filter transformation
		if len(client.Filters) > 0 {
			// Create a copy of the event data to modify for this client
			clientEventData := make(map[string]interface{})
			for k, v := range eventData {
				clientEventData[k] = v
			}
			
			// Check each filter to see if it's a specific field request
			for _, filter := range client.Filters {
				log.Printf("DEBUG: Processing filter %s against path %s", filter.Expression, path)
				
				// Check if this filter has conditions (key-value filters)
				hasConditions := len(filter.Conditions) > 0
				if hasConditions {
					log.Printf("DEBUG: Filter has %d conditions: %+v", len(filter.Conditions), filter.Conditions)
				}
				
				// Generic filtering approach for any data path
				// Case 1: If we're at the exact path the client is filtering for
				if path == filter.Path {
					// Already the exact path, no need to filter path further
					clientEventData["filtered"] = true
					
					// If there are conditions, we need to filter the data by those conditions
					if hasConditions {
						if filteredValue, success := applyKeyValueFilters(value, filter.Conditions); success {
							log.Printf("DEBUG: Applied key-value filtering to exact path match. Before: %T %+v, After: %T %+v", 
								value, value, filteredValue, filteredValue)
							clientEventData["value"] = filteredValue
							clientEventData["key_value_filtered"] = true
						}
					}
					
					log.Printf("DEBUG: Exact path match, filtering applied: %v", hasConditions)
					break
				}
				
				// Case 2: If the client filter is more specific than our current path
				// Example: client wants .data.offers but we're broadcasting .data
				if strings.HasPrefix(filter.Path, path) && len(filter.Path) > len(path) {
					// Need to extract just the part they want
					remainingPath := filter.Path[len(path):]
					if strings.HasPrefix(remainingPath, ".") {
						// If our path is a prefix of the filter path, try to extract the specific data
						// Example: extract only "offers" from "data" when filter is "data.offers"
						extractPath := remainingPath
						log.Printf("DEBUG: Filter is more specific than broadcast path. Extracting from %s using %s", path, extractPath)
						
						// Create a matcher to extract the specific field
						matcher := query.NewMatcher()
						
						// Try to get the specific field
						filteredValue, err := matcher.Get(value, extractPath)
						if err == nil {
							// Replace the full data with just the filtered data
							clientEventData["value"] = filteredValue
							clientEventData["filtered"] = true
							
							// If there are conditions, apply key-value filtering
							if hasConditions {
								if kv_filtered, success := applyKeyValueFilters(filteredValue, filter.Conditions); success {
									log.Printf("DEBUG: Applied key-value filtering to extracted path. Before: %T %+v, After: %T %+v", 
										filteredValue, filteredValue, kv_filtered, kv_filtered)
									clientEventData["value"] = kv_filtered
									clientEventData["key_value_filtered"] = true
								}
							}
							
							log.Printf("DEBUG: Successfully extracted specific value for %s", filter.Path)
							break
						} else {
							log.Printf("DEBUG: Failed to extract specific value: %v", err)
						}
					}
				}
				
				// Case 3: If we're broadcasting a more specific path than the client filter
				// Example: client wants .data but we're broadcasting .data.offers
				if strings.HasPrefix(path, filter.Path) && len(path) > len(filter.Path) {
					// This is already handled by ShouldNotify, but we mark it as filtered
					clientEventData["filtered"] = true
					
					// If there are conditions, we need to apply them
					if hasConditions {
						// Extract the field we're interested in
						fieldName := strings.TrimPrefix(path, filter.Path+".")
						
						// Apply key-value filtering to the data
						if filteredValue, success := applyKeyValueFilters(value, filter.Conditions); success {
							log.Printf("DEBUG: Applied key-value filtering to more specific path %s with field %s. Before: %T %+v, After: %T %+v", 
								path, fieldName, value, value, filteredValue, filteredValue)
							clientEventData["value"] = filteredValue
							clientEventData["key_value_filtered"] = true
						}
					}
					
					log.Printf("DEBUG: Broadcast path is more specific than filter, client will receive it")
					break
				}
				
				// Case 4: Specific handling for structured paths like .data.X
				// This handles cases where the paths don't strictly have a prefix relationship
				// but the value might contain the requested data
				if strings.HasPrefix(filter.Path, ".data.") && strings.HasPrefix(path, ".data") {
					// Extract what the client is looking for (after .data.)
					clientTarget := strings.TrimPrefix(filter.Path, ".data.")
					
					// Check if value has this specific field
					if valueMap, ok := value.(map[string]interface{}); ok {
						if data, ok := valueMap["data"].(map[string]interface{}); ok {
							// We have a data field in our value, check if it contains what client wants
							if targetValue, exists := data[clientTarget]; exists {
								log.Printf("DEBUG: Found direct match for %s in data map", clientTarget)
								
								// Get the target value
								filteredValue := targetValue
								
								// Apply key-value filtering if needed
								if hasConditions {
									if kv_filtered, success := applyKeyValueFilters(filteredValue, filter.Conditions); success {
										log.Printf("DEBUG: Applied key-value filtering to data field. Before: %T %+v, After: %T %+v", 
											filteredValue, filteredValue, kv_filtered, kv_filtered)
										filteredValue = kv_filtered
										clientEventData["key_value_filtered"] = true
									}
								}
								
								clientEventData["value"] = filteredValue
								clientEventData["filtered"] = true
								break
							}
						}
					}
				}
			}
			
			// Log the final data structure
			valueType := fmt.Sprintf("%T", clientEventData["value"])
			log.Printf("DEBUG: Final event data type: %s", valueType)
			
			// For array values, log the length and sample
			if array, ok := clientEventData["value"].([]interface{}); ok {
				log.Printf("DEBUG: Final array length: %d", len(array))
				if len(array) > 0 {
					log.Printf("DEBUG: First item sample: %+v", array[0])
				}
			}
			
			jsonData, _ := json.Marshal(clientEventData)
			log.Printf("DEBUG: Final event data to send: %s", string(jsonData))
			
			// Send the possibly modified event data
			client.Send(eventType, clientEventData)
		} else {
			// No filters, send the original event data
			client.Send(eventType, eventData)
		}
	}
}

// applyKeyValueFilters filters an array of items based on key-value conditions
func applyKeyValueFilters(data interface{}, conditions []query.KeyValueCondition) (interface{}, bool) {
	// If no conditions or no data, return as is
	if len(conditions) == 0 || data == nil {
		return data, false
	}
	
	// Handle array data
	if array, ok := data.([]interface{}); ok {
		// Create a new array to hold matching items
		var filtered []interface{}
		
		// Check each item against all conditions
		for _, item := range array {
			if mapItem, ok := item.(map[string]interface{}); ok {
				// Check if this item matches all conditions
				allMatch := true
				for _, condition := range conditions {
					if value, exists := mapItem[condition.Key]; exists {
						// Convert to string for comparison
						strValue := fmt.Sprintf("%v", value)
						if strings.TrimSpace(strValue) != condition.Value {
							allMatch = false
							break
						}
					} else {
						// Key doesn't exist
						allMatch = false
						break
					}
				}
				
				// If all conditions match, add this item to results
				if allMatch {
					filtered = append(filtered, item)
				}
			}
		}
		
		// Return the filtered results
		if len(filtered) > 0 {
			return filtered, true
		}
		
		// No matches, return empty array
		return []interface{}{}, true
	}
	
	// For non-array data, try to match directly
	if mapData, ok := data.(map[string]interface{}); ok {
		// Check all conditions
		for _, condition := range conditions {
			if value, exists := mapData[condition.Key]; exists {
				// Convert to string for comparison
				strValue := fmt.Sprintf("%v", value)
				if strings.TrimSpace(strValue) != condition.Value {
					// Doesn't match
					return data, false
				}
			} else {
				// Key doesn't exist
				return data, false
			}
		}
		
		// All conditions match
		return mapData, true
	}
	
	// Can't filter this type of data
	return data, false
}

// Helper function to get map keys for logging
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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

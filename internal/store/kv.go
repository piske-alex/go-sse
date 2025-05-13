package store

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"sync"
	
	"github.com/piske-alex/go-sse/internal/query"
)

// KVStore represents an in-memory key-value store with concurrency safety
type KVStore struct {
	data map[string]interface{}
	mux  sync.RWMutex
}

// NewStore creates a new empty KV store
func NewStore() *KVStore {
	return &KVStore{
		data: make(map[string]interface{}),
	}
}

// Initialize sets the initial data for the store
func (s *KVStore) Initialize(data map[string]interface{}) error {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.data = data
	return nil
}

// InitializeFromJSON initializes the store from a JSON byte array
func (s *KVStore) InitializeFromJSON(jsonData []byte) error {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}

	return s.Initialize(data)
}

// Get retrieves a value by path
func (s *KVStore) Get(path string) (interface{}, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	// If path is empty or ".", return the entire store
	if path == "" || path == "." {
		return s.data, nil
	}

	// Parse the path and navigate the store
	result, err := s.getValueByPath(s.data, path)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Set updates a value at the given path
func (s *KVStore) Set(path string, value interface{}) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// If path is empty or ".", replace the entire store
	if path == "" || path == "." {
		// Ensure value is a map
		valMap, ok := value.(map[string]interface{})
		if !ok {
			return errors.New("value must be a map when setting root")
		}
		s.data = valMap
		return nil
	}

	// Update the value at the specified path
	return s.setValueByPath(s.data, path, value)
}

// SetFromJSON updates a value at the given path from JSON
func (s *KVStore) SetFromJSON(path string, jsonData []byte) error {
	var value interface{}
	err := json.Unmarshal(jsonData, &value)
	if err != nil {
		return err
	}

	return s.Set(path, value)
}

// Delete removes a value at the given path
func (s *KVStore) Delete(path string) error {
	s.mux.Lock()
	defer s.mux.Unlock()

	// If path is empty or ".", reset the entire store
	if path == "" || path == "." {
		s.data = make(map[string]interface{})
		return nil
	}

	// Delete the value at the specified path
	return s.deleteByPath(s.data, path)
}

// ToJSON serializes the entire store to JSON
func (s *KVStore) ToJSON() ([]byte, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return json.Marshal(s.data)
}

// getValueByPath navigates the map using the provided path and returns the value
func (s *KVStore) getValueByPath(data map[string]interface{}, path string) (interface{}, error) {
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Get the value at the path
	result, err := matcher.Get(data, path)
	if err != nil {
		if err == query.ErrPathNotFound {
			return nil, ErrPathNotFound
		}
		return nil, err
	}
	
	return result, nil
}

// setValueByPath updates a value at the specified path
func (s *KVStore) setValueByPath(data map[string]interface{}, path string, value interface{}) error {
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Set the value at the path
	err := matcher.Set(data, path, value)
	if err != nil {
		if err == query.ErrPathNotFound {
			return ErrPathNotFound
		}
		return err
	}
	
	return nil
}

// deleteByPath removes a value at the specified path
func (s *KVStore) deleteByPath(data map[string]interface{}, path string) error {
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Delete the value at the path
	err := matcher.Delete(data, path)
	if err != nil {
		if err == query.ErrPathNotFound {
			return ErrPathNotFound
		}
		return err
	}
	
	return nil
}

// FindMatches finds all values matching a path expression
func (s *KVStore) FindMatches(path string) ([]query.MatchResult, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	
	// Log input for debugging
	log.Printf("KVStore.FindMatches called with path: %s", path)
	
	// Handle specific case for .data.positions
	if path == ".data.positions" || path == "data.positions" {
		// Try to directly get the value at the specific path
		var result []query.MatchResult
		
		// First try data.positions directly
		if data, ok := s.data["data"].(map[string]interface{}); ok {
			if positions, ok := data["positions"]; ok {
				log.Printf("Found positions at data.positions direct path")
				result = append(result, query.MatchResult{
					Path:  path,
					Value: positions,
				})
				return result, nil
			}
		}
		
		// Next try data as the root key
		if dataMap, ok := s.data["data"].(map[string]interface{}); ok {
			if positions, ok := dataMap["positions"]; ok {
				log.Printf("Found positions in data map")
				result = append(result, query.MatchResult{
					Path:  path,
					Value: positions,
				})
				return result, nil
			}
		}
	}
	
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Find matches
	results, err := matcher.Match(s.data, path)
	if err != nil {
		log.Printf("Matcher.Match error: %v", err)
		return nil, err
	}
	
	log.Printf("Found %d matches for path %s", len(results), path)
	return results, nil
}

// DisplayStoreInfo displays the contents of the in-memory store
func (s *KVStore) DisplayStoreInfo() error {
	s.mux.RLock()
	defer s.mux.RUnlock()
	
	log.Println("====== In-Memory Store Information ======")
	
	// Check if store is empty
	if len(s.data) == 0 {
		log.Println("Store is empty")
		log.Println("Collections: 0")
		log.Println("Documents: 0")
		log.Println("=====================================")
		return nil
	}
	
	// Convert data to JSON for nice display
	jsonData, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		log.Printf("Error marshaling store data: %v", err)
		return err
	}
	
	// Get the size of the data
	dataSizeKB := float64(len(jsonData)) / 1024.0
	
	// Count collections (top-level maps) and documents (entries in those maps)
	collections := 0
	documents := 0
	collectionDetails := make(map[string]int)

	// Show stats about the store
	log.Printf("Store size: %.2f KB", dataSizeKB)
	log.Printf("Top-level keys: %d", len(s.data))
	
	// List all top-level keys and identify collections
	log.Println("Top-level structure:")
	for key, value := range s.data {
		// For map values, consider them as collections
		if mapValue, ok := value.(map[string]interface{}); ok {
			collections++
			numDocs := len(mapValue)
			documents += numDocs
			collectionDetails[key] = numDocs
			log.Printf("  %s: collection with %d documents", key, numDocs)
			
			// Sample documents in this collection
			if numDocs > 0 {
				i := 0
				for docKey, docValue := range mapValue {
					if i >= 3 { // Only show first 3 documents
						log.Printf("    ... and %d more documents", numDocs-3)
						break
					}
					
					// Convert document to JSON for display
					docJson, err := json.Marshal(docValue)
					if err != nil {
						log.Printf("    %s: [error marshaling]", docKey)
					} else {
						docStr := string(docJson)
						if len(docStr) > 100 {
							docStr = docStr[:100] + "... (truncated)"
						}
						log.Printf("    %s: %s", docKey, docStr)
					}
					i++
				}
			}
		} else if sliceValue, ok := value.([]interface{}); ok {
			// For slice values, consider them as collections of documents
			collections++
			numDocs := len(sliceValue)
			documents += numDocs
			collectionDetails[key] = numDocs
			log.Printf("  %s: array collection with %d documents", key, numDocs)
			
			// Sample documents in this collection
			if numDocs > 0 {
				for i := 0; i < min(3, numDocs); i++ { // Only show first 3 elements
					// Convert document to JSON for display
					docJson, err := json.Marshal(sliceValue[i])
					if err != nil {
						log.Printf("    [%d]: [error marshaling]", i)
					} else {
						docStr := string(docJson)
						if len(docStr) > 100 {
							docStr = docStr[:100] + "... (truncated)"
						}
						log.Printf("    [%d]: %s", i, docStr)
					}
				}
				if numDocs > 3 {
					log.Printf("    ... and %d more documents", numDocs-3)
				}
			}
		} else {
			// For other values, show the type (these are not collections)
			log.Printf("  %s: %T value", key, value)
		}
	}
	
	// Show collection summary
	log.Printf("Collections found: %d", collections)
	log.Printf("Total documents: %d", documents)
	log.Println("Collection details:")
	if len(collectionDetails) == 0 {
		log.Println("  No collections found")
	} else {
		for coll, count := range collectionDetails {
			log.Printf("  %s: %d documents", coll, count)
		}
	}
	
	// Show the full data if it's not too large
	if dataSizeKB < 25 {
		log.Println("Full store contents:")
		log.Println(string(jsonData))
	} else {
		log.Printf("Store contents too large to display (%.2f KB). Showing sample:", dataSizeKB)
		// Sample the first 20 lines of the JSON
		lines := strings.Split(string(jsonData), "\n")
		sampleSize := 20
		if len(lines) < sampleSize {
			sampleSize = len(lines)
		}
		for i := 0; i < sampleSize; i++ {
			log.Println(lines[i])
		}
		log.Printf("... (truncated, %.2f KB total)", dataSizeKB)
	}
	
	log.Println("=====================================")
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

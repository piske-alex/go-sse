package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"regexp"
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

	// Extract key-value conditions from path if present
	var cleanPath string = path
	var keyValueConditions []string
	
	// Check if the path contains key-value conditions like [key=value]
	if strings.Contains(path, "[") && strings.Contains(path, "=") && strings.Contains(path, "]") {
		// Extract the condition part
		re := regexp.MustCompile(`\[([^=\[\]]+)=([^\[\]]+)\]`)
		matches := re.FindAllStringSubmatch(path, -1)
		
		if len(matches) > 0 {
			// Store the conditions for later use
			for _, match := range matches {
				if len(match) >= 3 {
					keyValueConditions = append(keyValueConditions, fmt.Sprintf("%s=%s", 
						strings.TrimSpace(match[1]), strings.TrimSpace(match[2])))
				}
			}
			
			// Clean the path by removing the conditions
			cleanPath = re.ReplaceAllString(path, "")
			log.Printf("Path with key-value condition detected. Original path: %s, Clean path: %s, Conditions: %v", 
				path, cleanPath, keyValueConditions)
		}
	}

	// Handle special case for .data.X paths with conditions
	if strings.HasPrefix(cleanPath, ".data.") || strings.HasPrefix(cleanPath, "data.") {
		// Extract the target field (like "positions", "offers", etc.)
		parts := strings.Split(cleanPath, ".")
		if len(parts) > 1 {
			targetField := parts[len(parts)-1]
			log.Printf("Special case handling for data.%s path", targetField)
			
			// Try to get the specific field from data
			if dataMap, ok := s.data["data"].(map[string]interface{}); ok {
				if fieldValue, exists := dataMap[targetField]; exists {
					log.Printf("Found %s in data map", targetField)
					
					// Apply key-value filtering if needed
					if len(keyValueConditions) > 0 {
						fieldValue = s.applyKeyValueFiltering(fieldValue, keyValueConditions)
						log.Printf("Applied key-value filtering to %s", targetField)
					}
					
					return fieldValue, nil
				}
			}
		}
	}

	// If path is empty or ".", return the entire store
	if cleanPath == "" || cleanPath == "." {
		return s.data, nil
	}

	// Parse the path and navigate the store
	result, err := s.getValueByPath(s.data, cleanPath)
	if err != nil {
		return nil, err
	}

	// Apply key-value filtering if needed
	if len(keyValueConditions) > 0 {
		result = s.applyKeyValueFiltering(result, keyValueConditions)
		log.Printf("Applied key-value filtering to result")
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
	
	// Extract key-value conditions from path if present
	var cleanPath string = path
	var keyValueConditions []string
	
	// Check if the path contains key-value conditions like [key=value]
	if strings.Contains(path, "[") && strings.Contains(path, "=") && strings.Contains(path, "]") {
		// Extract the condition part
		re := regexp.MustCompile(`\[([^=\[\]]+)=([^\[\]]+)\]`)
		matches := re.FindAllStringSubmatch(path, -1)
		
		if len(matches) > 0 {
			// Store the conditions for later use
			for _, match := range matches {
				if len(match) >= 3 {
					keyValueConditions = append(keyValueConditions, fmt.Sprintf("%s=%s", 
						strings.TrimSpace(match[1]), strings.TrimSpace(match[2])))
				}
			}
			
			// Clean the path by removing the conditions
			cleanPath = re.ReplaceAllString(path, "")
			log.Printf("Path with key-value condition detected in FindMatches. Original path: %s, Clean path: %s, Conditions: %v", 
				path, cleanPath, keyValueConditions)
		}
	}
	
	// Handle specific case for .data.X paths
	if strings.HasPrefix(cleanPath, ".data.") || strings.HasPrefix(cleanPath, "data.") {
		// Extract the field name after ".data."
		parts := strings.Split(cleanPath, ".")
		if len(parts) > 1 {
			targetField := parts[len(parts)-1]
			var result []query.MatchResult
			
			// First try getting the field from within "data"
			if data, ok := s.data["data"].(map[string]interface{}); ok {
				if fieldValue, ok := data[targetField]; ok {
					log.Printf("Found %s at data.%s direct path", targetField, targetField)
					
					// Apply key-value filtering if needed
					if len(keyValueConditions) > 0 {
						fieldValue = s.applyKeyValueFiltering(fieldValue, keyValueConditions)
						log.Printf("Applied key-value filtering to %s in FindMatches", targetField)
					}
					
					result = append(result, query.MatchResult{
						Path:  path, // Use original path with conditions
						Value: fieldValue,
					})
					return result, nil
				}
			}
		}
	}
	
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Find matches
	results, err := matcher.Match(s.data, cleanPath)
	if err != nil {
		log.Printf("Matcher.Match error: %v", err)
		return nil, err
	}
	
	// Apply key-value filtering if needed
	if len(keyValueConditions) > 0 {
		var filteredResults []query.MatchResult
		
		for _, result := range results {
			filteredValue := s.applyKeyValueFiltering(result.Value, keyValueConditions)
			
			// Skip empty array results (no matches)
			if array, ok := filteredValue.([]interface{}); ok && len(array) == 0 {
				continue
			}
			
			filteredResults = append(filteredResults, query.MatchResult{
				Path:  result.Path,
				Value: filteredValue,
			})
		}
		
		results = filteredResults
		log.Printf("Applied key-value filtering to %d matches", len(results))
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

// applyKeyValueFiltering filters data based on key-value conditions
func (s *KVStore) applyKeyValueFiltering(data interface{}, conditions []string) interface{} {
	// If no conditions or no data, return as is
	if len(conditions) == 0 || data == nil {
		return data
	}
	
	// Parse conditions into key-value pairs
	keyValuePairs := make(map[string]string)
	for _, cond := range conditions {
		parts := strings.Split(cond, "=")
		if len(parts) == 2 {
			keyValuePairs[parts[0]] = parts[1]
		}
	}
	
	// For array data, filter the items
	if array, ok := data.([]interface{}); ok {
		// Create a new array to hold matching items
		var filtered []interface{}
		
		// Filter items based on conditions
		for _, item := range array {
			if mapItem, ok := item.(map[string]interface{}); ok {
				// Check if this item matches all conditions
				allMatch := true
				for key, value := range keyValuePairs {
					if fieldValue, ok := mapItem[key]; ok {
						// Convert value to string for comparison
						strValue := fmt.Sprintf("%v", fieldValue)
						if strings.TrimSpace(strValue) != value {
							allMatch = false
							break
						}
					} else {
						// Key doesn't exist
						allMatch = false
						break
					}
				}
				
				// Add matching items to result
				if allMatch {
					filtered = append(filtered, item)
				}
			}
		}
		
		// Return filtered result (or empty array if no matches)
		return filtered
	}
	
	// For non-array data, return as-is
	return data
}

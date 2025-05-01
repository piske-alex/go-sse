package store

import (
	"encoding/json"
	"errors"
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
	
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Find matches
	results, err := matcher.Match(s.data, path)
	if err != nil {
		return nil, err
	}
	
	return results, nil
}

package store

import (
	"encoding/json"
	"errors"
	"sync"
)

// ErrPathNotFound is returned when a path cannot be found in the store
var ErrPathNotFound = errors.New("path not found in store")

// Store represents an in-memory key-value store with concurrency safety
type Store struct {
	data map[string]interface{}
	mux  sync.RWMutex
}

// NewStore creates a new empty KV store
func NewStore() *Store {
	return &Store{
		data: make(map[string]interface{}),
	}
}

// Initialize sets the initial data for the store
func (s *Store) Initialize(data map[string]interface{}) {
	s.mux.Lock()
	defer s.mux.Unlock()
	s.data = data
}

// InitializeFromJSON initializes the store from a JSON byte array
func (s *Store) InitializeFromJSON(jsonData []byte) error {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}

	s.Initialize(data)
	return nil
}

// Get retrieves a value by path
func (s *Store) Get(path string) (interface{}, error) {
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
func (s *Store) Set(path string, value interface{}) error {
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
func (s *Store) SetFromJSON(path string, jsonData []byte) error {
	var value interface{}
	err := json.Unmarshal(jsonData, &value)
	if err != nil {
		return err
	}

	return s.Set(path, value)
}

// Delete removes a value at the given path
func (s *Store) Delete(path string) error {
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
func (s *Store) ToJSON() ([]byte, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()

	return json.Marshal(s.data)
}

// getValueByPath navigates the map using the provided path and returns the value
func (s *Store) getValueByPath(data map[string]interface{}, path string) (interface{}, error) {
	// TODO: Implement path navigation based on the query package
	// This is a placeholder until we implement the JQ-style path processor
	return nil, errors.New("path navigation not implemented yet")
}

// setValueByPath updates a value at the specified path
func (s *Store) setValueByPath(data map[string]interface{}, path string, value interface{}) error {
	// TODO: Implement path navigation and update based on the query package
	// This is a placeholder until we implement the JQ-style path processor
	return errors.New("path navigation not implemented yet")
}

// deleteByPath removes a value at the specified path
func (s *Store) deleteByPath(data map[string]interface{}, path string) error {
	// TODO: Implement path navigation and deletion based on the query package
	// This is a placeholder until we implement the JQ-style path processor
	return errors.New("path navigation not implemented yet")
}

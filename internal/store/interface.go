package store

import (
	"errors"

	"github.com/piske-alex/go-sse/internal/query"
)

// ErrPathNotFound is returned when a path cannot be found in the store
var ErrPathNotFound = errors.New("path not found in store")

// Store defines the interface for a key-value store that supports JQ-style paths
type Store interface {
	// Initialize sets the initial data for the store
	Initialize(data map[string]interface{}) error

	// InitializeFromJSON initializes the store from a JSON byte array
	InitializeFromJSON(jsonData []byte) error

	// Get retrieves a value by path
	Get(path string) (interface{}, error)

	// Set updates a value at the given path
	Set(path string, value interface{}) error

	// SetFromJSON updates a value at the given path from JSON
	SetFromJSON(path string, jsonData []byte) error

	// Delete removes a value at the given path
	Delete(path string) error

	// ToJSON serializes the entire store to JSON
	ToJSON() ([]byte, error)

	// FindMatches finds all values matching a path expression
	FindMatches(path string) ([]query.MatchResult, error)
	
	// DisplayStoreInfo displays information about the store contents
	DisplayStoreInfo() error
}

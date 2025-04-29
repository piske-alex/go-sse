package store

import (
	"fmt"
	"os"

	"github.com/piske-alex/go-sse/internal/query"
)

// StoreType defines the type of store to create
type StoreType string

const (
	// MemoryStore is an in-memory key-value store
	MemoryStore StoreType = "memory"
	// MongoStore is a MongoDB-backed store
	MongoStore StoreType = "mongo"
)

// StoreInterface defines the interface for a key-value store
type StoreInterface interface {
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
}

// CreateStore creates a store of the specified type
func CreateStore(storeType StoreType) (interface{}, error) {
	switch storeType {
	case MemoryStore:
		return NewStore(), nil

	case MongoStore:
		// Get MongoDB connection details from environment variables
		uri := os.Getenv("MONGO_URI")
		if uri == "" {
			uri = "mongodb://localhost:27017"
		}

		dbName := os.Getenv("MONGO_DB_NAME")
		if dbName == "" {
			dbName = "gosse"
		}

		collectionName := os.Getenv("MONGO_COLLECTION")
		if collectionName == "" {
			collectionName = "kv_store"
		}

		documentID := os.Getenv("MONGO_DOCUMENT_ID")
		if documentID == "" {
			documentID = "main"
		}

		return NewMongoStore(uri, dbName, collectionName, documentID)

	default:
		return nil, fmt.Errorf("unknown store type: %s", storeType)
	}
}

package store

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/piske-alex/go-sse/internal/query"
)

// StoreType defines the type of store to create
type StoreType string

const (
	// MemoryStore is an in-memory key-value store
	MemoryStore StoreType = "memory"
	// MongoStoreType is a MongoDB-backed store
	MongoStoreType StoreType = "mongo"
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

// BuildMongoURI constructs a MongoDB connection URI from individual components or uses a complete URI
func BuildMongoURI() string {
	// Check if a complete URI is provided
	uri := os.Getenv("MONGO_URI")
	if uri != "" {
		// If URI already contains credentials, use it directly
		if strings.Contains(uri, "@") {
			log.Println("Using fully configured MongoDB URI from MONGO_URI")
			return uri
		}
		
		// If URI doesn't contain credentials, check for separate username/password
		user := os.Getenv("MONGO_USER")
		pass := os.Getenv("MONGO_PASSWORD")
		
		if user != "" && pass != "" {
			// Extract the protocol and host parts
			parts := strings.SplitN(uri, "://", 2)
			if len(parts) != 2 {
				log.Println("MONGO_URI format not recognized, using as-is")
				return uri
			}
			
			protocol := parts[0]
			host := parts[1]
			
			// Construct URI with credentials
			uri = fmt.Sprintf("%s://%s:%s@%s", protocol, user, pass, host)
			log.Println("Built MongoDB URI with credentials from MONGO_USER and MONGO_PASSWORD")
			return uri
		}
		
		// No credentials provided, use URI as-is
		log.Println("Using MongoDB URI without authentication")
		return uri
	}
	
	// No URI provided, build one from individual components
	host := os.Getenv("MONGO_HOST")
	port := os.Getenv("MONGO_PORT")
	user := os.Getenv("MONGO_USER")
	pass := os.Getenv("MONGO_PASSWORD")
	auth := os.Getenv("MONGO_AUTH_DB")
	
	// Set defaults
	if host == "" {
		host = "localhost"
	}
	
	if port == "" {
		port = "27017"
	}
	
	if auth == "" {
		auth = "admin"
	}
	
	// Build the URI
	if user != "" && pass != "" {
		// With authentication
		uri = fmt.Sprintf("mongodb://%s:%s@%s:%s/?authSource=%s", 
			user, pass, host, port, auth)
		log.Println("Built MongoDB URI with credentials from individual components")
	} else {
		// Without authentication
		uri = fmt.Sprintf("mongodb://%s:%s", host, port)
		log.Println("Built MongoDB URI without authentication from individual components")
	}
	
	return uri
}

// CreateStore creates a store of the specified type
func CreateStore(storeType StoreType) (Store, error) {
	switch storeType {
	case MemoryStore:
		return NewStore(), nil

	case MongoStoreType:
		// Build the MongoDB URI with proper authentication
		uri := BuildMongoURI()

		dbName := os.Getenv("MONGO_DB_NAME")
		if dbName == "" {
			dbName = "test"
		}

		collectionName := os.Getenv("MONGO_COLLECTION")
		if collectionName == "" {
			collectionName = "sse"
		}

		// Check for the MONGO_USE_COLLECTION_ROOT environment variable
		// If it's set to "true", we'll use the collection as the root instead of a document
		useCollectionRoot := os.Getenv("MONGO_USE_COLLECTION_ROOT")
		documentID := os.Getenv("MONGO_DOCUMENT_ID")

		// Empty documentID or "collection" means use collection as root
		if useCollectionRoot == "true" || useCollectionRoot == "1" {
			log.Println("Using MongoDB collection as root path (collection-based document store)")
			documentID = "collection" // Special value to trigger collection mode
		} else if documentID == "" {
			documentID = "latest" // Default document ID
			log.Println("Using document-based MongoDB store with document ID:", documentID)
		} else {
			log.Println("Using document-based MongoDB store with document ID:", documentID)
		}

		return NewMongoStore(uri, dbName, collectionName, documentID)

	default:
		return nil, fmt.Errorf("unknown store type: %s", storeType)
	}
}

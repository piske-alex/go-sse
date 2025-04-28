package store

import (
	"context"
	"errors"
	"time"

	"github.com/piske-alex/go-sse/internal/query"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoStore implements the Store interface using MongoDB as backend
type MongoStore struct {
	client         *mongo.Client
	database       *mongo.Database
	collection     *mongo.Collection
	documentID     string // The ID of the main document
	defaultTimeout time.Duration
	matcher        *query.Matcher
}

// MongoConfig holds configuration for MongoDB connection
type MongoConfig struct {
	URI          string
	Database     string
	Collection   string
	DocumentID   string        // ID to use for the main document
	Timeout      time.Duration // Default timeout for operations
	ConnPoolSize uint64        // Connection pool size
}

// NewDefaultMongoConfig returns a default MongoDB configuration
func NewDefaultMongoConfig() *MongoConfig {
	return &MongoConfig{
		URI:          "mongodb://localhost:27017",
		Database:     "sse",
		Collection:   "store",
		DocumentID:   "main", // Single document to store all data
		Timeout:      5 * time.Second,
		ConnPoolSize: 100,
	}
}

// NewMongoStore creates a new MongoDB-backed store
func NewMongoStore(config *MongoConfig) (*MongoStore, error) {
	if config == nil {
		config = NewDefaultMongoConfig()
	}

	// Create MongoDB client
	clientOptions := options.Client().ApplyURI(config.URI)
	
	// Set connection pool size
	clientOptions.SetMaxPoolSize(config.ConnPoolSize)
	
	// Set timeouts
	clientOptions.SetConnectTimeout(config.Timeout)
	
	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the database to verify connection
	ctx, cancel = context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	// Get database and collection
	database := client.Database(config.Database)
	collection := database.Collection(config.Collection)

	// Return the store
	return &MongoStore{
		client:         client,
		database:       database,
		collection:     collection,
		documentID:     config.DocumentID,
		defaultTimeout: config.Timeout,
		matcher:        query.NewMatcher(),
	}, nil
}

// Close closes the MongoDB connection
func (s *MongoStore) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.defaultTimeout)
	defer cancel()
	
	return s.client.Disconnect(ctx)
}

// Initialize sets the initial data for the store
func (s *MongoStore) Initialize(data map[string]interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.defaultTimeout)
	defer cancel()

	// Create a document with our ID and the data
	document := bson.M{
		"_id":  s.documentID,
		"data": data,
	}

	// Use upsert to replace any existing document or create a new one
	opts := options.Replace().SetUpsert(true)
	_, err := s.collection.ReplaceOne(ctx, bson.M{"_id": s.documentID}, document, opts)

	return err
}

// InitializeFromJSON initializes the store from a JSON byte array
func (s *MongoStore) InitializeFromJSON(jsonData []byte) error {
	var data map[string]interface{}
	err := bson.UnmarshalExtJSON(jsonData, true, &data)
	if err != nil {
		return err
	}

	return s.Initialize(data)
}

// Get retrieves a value by path
func (s *MongoStore) Get(path string) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), s.defaultTimeout)
	defer cancel()

	// If path is empty or ".", return the entire store
	if path == "" || path == "." {
		result := s.collection.FindOne(ctx, bson.M{"_id": s.documentID})
		if result.Err() != nil {
			if errors.Is(result.Err(), mongo.ErrNoDocuments) {
				return nil, ErrPathNotFound
			}
			return nil, result.Err()
		}

		var document bson.M
		err := result.Decode(&document)
		if err != nil {
			return nil, err
		}

		// Return just the data part
		return document["data"], nil
	}

	// Get the entire document first
	data, err := s.Get(".")
	if err != nil {
		return nil, err
	}
	
	// Use the matcher to get the value at the specified path
	result, err := s.matcher.Get(data, path)
	if err != nil {
		return nil, ErrPathNotFound
	}
	
	return result, nil
}

// Set updates a value at the given path
func (s *MongoStore) Set(path string, value interface{}) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.defaultTimeout)
	defer cancel()

	// If path is empty or ".", replace the entire store
	if path == "" || path == "." {
		// Ensure value is a map
		valMap, ok := value.(map[string]interface{})
		if !ok {
			return errors.New("value must be a map when setting root")
		}
		return s.Initialize(valMap)
	}

	// For non-root paths, we need to get the document, modify it, and save it back
	// This is done atomically using MongoDB's update operators
	
	// Parse the path to MongoDB dot notation
	dotPath := s.convertToDotNotation(path)
	if dotPath == "" {
		return errors.New("invalid path")
	}
	
	// Update the document atomically
	_, err := s.collection.UpdateOne(
		ctx,
		bson.M{"_id": s.documentID},
		bson.M{"$set": bson.M{dotPath: value}},
	)
	
	return err
}

// SetFromJSON updates a value at the given path from JSON
func (s *MongoStore) SetFromJSON(path string, jsonData []byte) error {
	var value interface{}
	err := bson.UnmarshalExtJSON(jsonData, true, &value)
	if err != nil {
		return err
	}

	return s.Set(path, value)
}

// Delete removes a value at the given path
func (s *MongoStore) Delete(path string) error {
	ctx, cancel := context.WithTimeout(context.Background(), s.defaultTimeout)
	defer cancel()

	// If path is empty or ".", reset the entire store
	if path == "" || path == "." {
		_, err := s.collection.DeleteOne(ctx, bson.M{"_id": s.documentID})
		return err
	}

	// For non-root paths, remove the field using unset
	dotPath := s.convertToDotNotation(path)
	if dotPath == "" {
		return errors.New("invalid path")
	}
	
	// Update the document atomically
	_, err := s.collection.UpdateOne(
		ctx,
		bson.M{"_id": s.documentID},
		bson.M{"$unset": bson.M{dotPath: ""}},
	)
	
	return err
}

// ToJSON serializes the entire store to JSON
func (s *MongoStore) ToJSON() ([]byte, error) {
	data, err := s.Get(".")
	if err != nil {
		return nil, err
	}
	
	return bson.MarshalExtJSON(data, true, false)
}

// FindMatches finds all values matching a path expression
func (s *MongoStore) FindMatches(path string) ([]query.MatchResult, error) {
	// Get the entire document first
	data, err := s.Get(".")
	if err != nil {
		return nil, err
	}
	
	// Use the matcher to find matches
	return s.matcher.Match(data, path)
}

// convertToDotNotation converts our JQ-style path to MongoDB dot notation
// e.g., ".users[0].name" -> "data.users.0.name"
func (s *MongoStore) convertToDotNotation(path string) string {
	// Parse the path using our query parser
	segments, err := s.matcher.Parser.Parse(path)
	if err != nil {
		return ""
	}
	
	// Start with "data" since our actual data is under the "data" field
	result := "data"
	
	// Skip the first segment (root)
	for i := 1; i < len(segments); i++ {
		segment := segments[i]
		
		switch segment.Type {
		case query.Property:
			result += "." + segment.Value
		case query.Index:
			result += "." + string(segment.Index)
		case query.Wildcard:
			// MongoDB doesn't directly support wildcards in update paths
			// For wildcards, we'd need to do multiple operations
			return ""
		default:
			return ""
		}
	}
	
	return result
}

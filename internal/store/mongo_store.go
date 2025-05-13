package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/piske-alex/go-sse/internal/query"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

// Document represents the structure of our MongoDB document
type Document struct {
	ID   string                 `bson:"_id"`
	Data map[string]interface{} `bson:"data"`
}

// MongoStore implements the Store interface using MongoDB as the backend
type MongoStore struct {
	client         *mongo.Client
	database       *mongo.Database
	collection     *mongo.Collection
	documentID     string    // This can be empty when using collection as root
	useCollection  bool      // When true, collection is root path and documentID is ignored
	context        context.Context
	cancelFunc     context.CancelFunc
	changeListener func(path string, value interface{})
}

// NewMongoStore creates a new MongoDB-backed store
func NewMongoStore(uri, dbName, collectionName, documentID string) (*MongoStore, error) {
	// Create a context with timeout for initial connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create MongoDB client
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}

	// Ping the MongoDB server to verify connection
	err = client.Ping(ctx, readpref.Primary())
	if err != nil {
		// Disconnect if ping fails
		client.Disconnect(ctx)
		return nil, err
	}

	// Create a background context for the store
	bgCtx, bgCancel := context.WithCancel(context.Background())

	// Determine if we're using collection as root based on documentID
	useCollection := documentID == "" || documentID == "collection"

	// Create the store
	store := &MongoStore{
		client:        client,
		database:      client.Database(dbName),
		collection:    client.Database(dbName).Collection(collectionName),
		documentID:    documentID,
		useCollection: useCollection,
		context:       bgCtx,
		cancelFunc:    bgCancel,
	}

	// Log the mode we're running in
	if useCollection {
		log.Printf("MongoDB store initialized with collection '%s' as root path", collectionName)
	} else {
		log.Printf("MongoDB store initialized with document ID '%s' as root path", documentID)
	}

	// Set up the change stream listener (only for document mode)
	if !useCollection {
		go store.setupChangeStream()
	}

	return store, nil
}

// setupChangeStream creates a MongoDB change stream to listen for updates
func (s *MongoStore) setupChangeStream() {
	// Skip if no change listener is set
	if s.changeListener == nil {
		return
	}

	// Create a pipeline that filters for document changes
	var pipeline mongo.Pipeline
	
	if s.useCollection {
		// In collection mode, watch all document changes in the collection
		pipeline = mongo.Pipeline{
			bson.D{
				{"$match", bson.D{
					{"operationType", bson.D{
						{"$in", bson.A{"update", "replace", "insert", "delete"}},
					}},
				}},
			},
		}
	} else {
		// In document mode, watch only our specific document
		pipeline = mongo.Pipeline{
			bson.D{
				{"$match", bson.D{
					{"operationType", bson.D{
						{"$in", bson.A{"update", "replace", "insert"}},
					}},
					{"documentKey._id", s.documentID},
				}},
			},
		}
	}

	// Create options with full document return
	opts := options.ChangeStream().SetFullDocument(options.UpdateLookup)

	// Start the change stream
	changeStream, err := s.collection.Watch(s.context, pipeline, opts)
	if err != nil {
		log.Printf("Error setting up change stream: %v", err)
		return
	}
	defer changeStream.Close(s.context)

	log.Printf("MongoDB change stream set up for %s mode", 
		map[bool]string{true: "collection", false: "document"}[s.useCollection])

	// Process change events
	for changeStream.Next(s.context) {
		// Decode the change event
		var changeEvent bson.M
		if err := changeStream.Decode(&changeEvent); err != nil {
			log.Printf("Error decoding change event: %v", err)
			continue
		}

		// Handle changes based on store mode
		if s.useCollection {
			// Collection mode - get document ID and report change for that document
			operationType, _ := changeEvent["operationType"].(string)
			
			// Get the document ID
			var docID string
			if documentKey, ok := changeEvent["documentKey"].(bson.M); ok {
				if id, ok := documentKey["_id"]; ok {
					docID = fmt.Sprintf("%v", id)
				}
			}
			
			if docID == "" {
				continue // Skip if no document ID
			}
			
			if operationType == "delete" {
				// For delete operations, notify with null value
				if s.changeListener != nil {
					s.changeListener(docID, nil)
				}
				continue
			}
			
			// For other operations, get the full document
			fullDocument, ok := changeEvent["fullDocument"].(bson.M)
			if !ok {
				continue
			}
			
			// Notify with the full document
			if s.changeListener != nil {
				s.changeListener(docID, fullDocument)
			}
		} else {
			// Document mode - extract data field from our document
			fullDocument, ok := changeEvent["fullDocument"].(bson.M)
			if !ok {
				continue
			}

			// Extract the data field
			data, ok := fullDocument["data"].(bson.M)
			if !ok {
				continue
			}

			// Convert to map[string]interface{}
			jsonData, err := json.Marshal(data)
			if err != nil {
				continue
			}

			var dataMap map[string]interface{}
			if err := json.Unmarshal(jsonData, &dataMap); err != nil {
				continue
			}

			// Notify the change listener
			if s.changeListener != nil {
				s.changeListener(".", dataMap)
			}
		}
	}

	if err := changeStream.Err(); err != nil {
		log.Printf("Change stream error: %v", err)
	}
}

// SetChangeListener sets a callback function that will be called when the data changes
func (s *MongoStore) SetChangeListener(listener func(path string, value interface{})) {
	s.changeListener = listener
}

// Initialize sets the initial data for the store
func (s *MongoStore) Initialize(data map[string]interface{}) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create the document
	doc := Document{
		ID:   s.documentID,
		Data: data,
	}

	// Use upsert to create or replace the document
	_, err := s.collection.ReplaceOne(
		ctx,
		bson.M{"_id": s.documentID},
		doc,
		options.Replace().SetUpsert(true),
	)

	return err
}

// InitializeFromJSON initializes the store from a JSON byte array
func (s *MongoStore) InitializeFromJSON(jsonData []byte) error {
	var data map[string]interface{}
	err := json.Unmarshal(jsonData, &data)
	if err != nil {
		return err
	}

	return s.Initialize(data)
}

// Get retrieves a value by path
func (s *MongoStore) Get(path string) (interface{}, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Different handling based on mode
	if s.useCollection {
		// In collection mode, use MongoDB query directly
		
		// If path is empty or ".", return all documents in collection
		if path == "" || path == "." {
			cursor, err := s.collection.Find(ctx, bson.M{})
			if err != nil {
				return nil, err
			}
			defer cursor.Close(ctx)
			
			var results []bson.M
			if err := cursor.All(ctx, &results); err != nil {
				return nil, err
			}
			
			// Convert to map with ID as key for better compatibility
			resultMap := make(map[string]interface{})
			for _, doc := range results {
				if id, ok := doc["_id"]; ok {
					// Convert _id to string for consistent usage
					idStr := fmt.Sprintf("%v", id)
					resultMap[idStr] = doc
				}
			}
			
			return resultMap, nil
		}
		
		// Check if path refers to a specific document ID
		// Try simple ID lookup first (root document names)
		var doc bson.M
		err := s.collection.FindOne(ctx, bson.M{"_id": path}).Decode(&doc)
		if err == nil {
			return doc, nil
		}
		
		// If not direct ID, try to parse the path for nested document access
		parts := strings.Split(path, ".")
		if len(parts) > 0 {
			// First part might be document ID, rest is path within document
			docID := parts[0]
			var doc bson.M
			err := s.collection.FindOne(ctx, bson.M{"_id": docID}).Decode(&doc)
			if err == nil {
				// If only document ID, return whole document
				if len(parts) == 1 {
					return doc, nil
				}
				
				// Otherwise, navigate nested path
				subPath := strings.Join(parts[1:], ".")
				matcher := query.NewMatcher()
				result, err := matcher.Get(doc, subPath)
				if err != nil {
					if err == query.ErrPathNotFound {
						return nil, ErrPathNotFound
					}
					return nil, err
				}
				return result, nil
			}
			
			// If docID lookup failed, try query with path as filter
			filter := bson.M{}
			// Try standard MongoDB dot notation query
			cursor, err := s.collection.Find(ctx, filter)
			if err != nil {
				return nil, err
			}
			defer cursor.Close(ctx)
			
			var results []bson.M
			if err := cursor.All(ctx, &results); err != nil {
				return nil, err
			}
			
			if len(results) == 0 {
				return nil, ErrPathNotFound
			}
			
			// For collection paths, always return as a map by ID
			resultMap := make(map[string]interface{})
			for _, doc := range results {
				if id, ok := doc["_id"]; ok {
					idStr := fmt.Sprintf("%v", id)
					resultMap[idStr] = doc
				}
			}
			
			return resultMap, nil
		}
		
		return nil, ErrPathNotFound
	} else {
		// Document mode - original implementation
		// Get the document
		var doc Document
		err := s.collection.FindOne(ctx, bson.M{"_id": s.documentID}).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, ErrPathNotFound
			}
			return nil, err
		}

		// If path is empty or ".", return the entire data
		if path == "" || path == "." {
			return doc.Data, nil
		}

		// Parse the path and navigate the data
		matcher := query.NewMatcher()
		result, err := matcher.Get(doc.Data, path)
		if err != nil {
			if err == query.ErrPathNotFound {
				return nil, ErrPathNotFound
			}
			return nil, err
		}

		return result, nil
	}
}

// Set updates a value at the given path
func (s *MongoStore) Set(path string, value interface{}) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Different handling based on mode
	if s.useCollection {
		// In collection mode
		
		// If path is empty or ".", replace the entire collection
		if path == "" || path == "." {
			// For safety, require a map for replacing collection
			docs, ok := value.(map[string]interface{})
			if !ok {
				return errors.New("value must be a map of documents when setting collection root")
			}
			
			// Collection replacement is a multi-step operation
			// 1. Delete all existing documents
			_, err := s.collection.DeleteMany(ctx, bson.M{})
			if err != nil {
				return err
			}
			
			// 2. Insert all new documents
			for key, val := range docs {
				// Make sure each document has an _id field
				docMap, ok := val.(map[string]interface{})
				if !ok {
					// If not a map, wrap it in a document with the key as ID
					docMap = map[string]interface{}{
						"_id":   key,
						"value": val,
					}
				} else {
					// If already a map, ensure it has _id
					if _, hasID := docMap["_id"]; !hasID {
						docMap["_id"] = key
					}
				}
				
				// Insert the document
				_, err := s.collection.InsertOne(ctx, docMap)
				if err != nil {
					return err
				}
			}
			
			return nil
		}
		
		// Check if path refers to a document (no dot)
		if !strings.Contains(path, ".") {
			// Path is document ID
			docMap, ok := value.(map[string]interface{})
			if !ok {
				// Wrap non-map values
				docMap = map[string]interface{}{
					"_id":   path, 
					"value": value,
				}
			} else {
				// Ensure document has _id field
				docMap["_id"] = path
			}
			
			// Upsert the document
			_, err := s.collection.ReplaceOne(
				ctx,
				bson.M{"_id": path},
				docMap,
				options.Replace().SetUpsert(true),
			)
			return err
		}
		
		// Handle dot notation - document.field.subfield
		parts := strings.Split(path, ".")
		if len(parts) > 1 {
			docID := parts[0]
			subPath := strings.Join(parts[1:], ".")
			
			// Get current document
			var doc bson.M
			err := s.collection.FindOne(ctx, bson.M{"_id": docID}).Decode(&doc)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					// Create new document with this path
					newDoc := bson.M{"_id": docID}
					
					// Create nested structure following the path
					current := newDoc
					for _, part := range parts[1:len(parts)-1] {
						current[part] = bson.M{}
						current = current[part].(bson.M)
					}
					
					// Set the value at the final path
					current[parts[len(parts)-1]] = value
					
					// Insert the document
					_, err := s.collection.InsertOne(ctx, newDoc)
					return err
				}
				return err
			}
			
			// Document exists, update field
			updateDoc := bson.M{"$set": bson.M{subPath: value}}
			_, err = s.collection.UpdateOne(ctx, bson.M{"_id": docID}, updateDoc)
			return err
		}
		
		return errors.New("invalid path format")
	} else {
		// Document mode - original implementation
		// First, get the document
		var doc Document
		err := s.collection.FindOne(ctx, bson.M{"_id": s.documentID}).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// Document doesn't exist, create a new one
				if path == "" || path == "." {
					// If path is root, simply store the value as is (if it's a map)
					valueMap, ok := value.(map[string]interface{})
					if !ok {
						return errors.New("value must be a map when setting root")
					}
					doc = Document{
						ID:   s.documentID,
						Data: valueMap,
					}
				} else {
					// Otherwise, create an empty map and set the value at the path
					doc = Document{
						ID:   s.documentID,
						Data: make(map[string]interface{}),
					}
					matcher := query.NewMatcher()
					err = matcher.Set(doc.Data, path, value)
					if err != nil {
						return err
					}
				}
			} else {
				return err
			}
		} else {
			// Document exists, update it at the specified path
			if path == "" || path == "." {
				// If path is root, simply replace the entire data (if it's a map)
				valueMap, ok := value.(map[string]interface{})
				if !ok {
					return errors.New("value must be a map when setting root")
				}
				doc.Data = valueMap
			} else {
				// Otherwise, set the value at the specified path
				matcher := query.NewMatcher()
				err = matcher.Set(doc.Data, path, value)
				if err != nil {
					return err
				}
			}
		}

		// Update or insert the document
		_, err = s.collection.ReplaceOne(
			ctx,
			bson.M{"_id": s.documentID},
			doc,
			options.Replace().SetUpsert(true),
		)

		return err
	}
}

// SetFromJSON updates a value at the given path from JSON
func (s *MongoStore) SetFromJSON(path string, jsonData []byte) error {
	var value interface{}
	err := json.Unmarshal(jsonData, &value)
	if err != nil {
		return err
	}

	return s.Set(path, value)
}

// Delete removes a value at the given path
func (s *MongoStore) Delete(path string) error {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Different handling based on mode
	if s.useCollection {
		// In collection mode
		
		// If path is empty or ".", delete all documents
		if path == "" || path == "." {
			_, err := s.collection.DeleteMany(ctx, bson.M{})
			return err
		}
		
		// Check if path refers to a document (no dot)
		if !strings.Contains(path, ".") {
			// Delete document by ID
			_, err := s.collection.DeleteOne(ctx, bson.M{"_id": path})
			return err
		}
		
		// Handle dot notation - document.field.subfield
		parts := strings.Split(path, ".")
		if len(parts) > 1 {
			docID := parts[0]
			subPath := strings.Join(parts[1:], ".")
			
			// Unset the field
			updateDoc := bson.M{"$unset": bson.M{subPath: ""}}
			_, err := s.collection.UpdateOne(ctx, bson.M{"_id": docID}, updateDoc)
			return err
		}
		
		return errors.New("invalid path format")
	} else {
		// Document mode - original implementation
		// If path is empty or ".", delete the entire document
		if path == "" || path == "." {
			_, err := s.collection.DeleteOne(ctx, bson.M{"_id": s.documentID})
			return err
		}

		// Otherwise, update the document by removing the value at the specified path
		// First, get the current document
		var doc Document
		err := s.collection.FindOne(ctx, bson.M{"_id": s.documentID}).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// Document doesn't exist, nothing to delete
				return nil
			}
			return err
		}

		// Delete the value at the specified path
		matcher := query.NewMatcher()
		err = matcher.Delete(doc.Data, path)
		if err != nil {
			if err == query.ErrPathNotFound {
				// Path doesn't exist, nothing to delete
				return nil
			}
			return err
		}

		// Update the document
		_, err = s.collection.ReplaceOne(ctx, bson.M{"_id": s.documentID}, doc)
		return err
	}
}

// ToJSON serializes the entire store to JSON
func (s *MongoStore) ToJSON() ([]byte, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Different handling based on mode
	if s.useCollection {
		// In collection mode, get all documents
		cursor, err := s.collection.Find(ctx, bson.M{})
		if err != nil {
			return nil, err
		}
		defer cursor.Close(ctx)
		
		// Decode all documents
		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			return nil, err
		}
		
		// Create a map with document IDs as keys for better compatibility with the rest of the code
		resultMap := make(map[string]interface{})
		for _, doc := range results {
			if id, ok := doc["_id"]; ok {
				// Convert ID to string for key
				idStr := fmt.Sprintf("%v", id)
				resultMap[idStr] = doc
			}
		}
		
		// Serialize the map to JSON
		return json.Marshal(resultMap)
	} else {
		// Document mode - original implementation
		// Get the document
		var doc Document
		err := s.collection.FindOne(ctx, bson.M{"_id": s.documentID}).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// If document doesn't exist, return an empty JSON object
				return []byte("{}"), nil
			}
			return nil, err
		}

		// Serialize the data to JSON
		return json.Marshal(doc.Data)
	}
}

// FindMatches finds all values matching a path expression
func (s *MongoStore) FindMatches(path string) ([]query.MatchResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if s.useCollection {
		// Log the incoming path for debugging
		log.Printf("MongoStore.FindMatches called with path: %s", path)
		
		// Clean up the path
		cleanPath := strings.TrimPrefix(path, ".data.")
		cleanPath = strings.TrimPrefix(cleanPath, "data.")
		log.Printf("Cleaned path: %s", cleanPath)

		// Create a projection to get only the specific field
		projection := bson.M{
			fmt.Sprintf("data.%s", cleanPath): 1,
		}

		var result bson.M
		err := s.collection.FindOne(
			ctx,
			bson.M{},
			options.FindOne().SetProjection(projection),
		).Decode(&result)

		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Printf("No documents found for path: %s", path)
				return []query.MatchResult{}, nil
			}
			log.Printf("Error finding documents for path %s: %v", path, err)
			return nil, err
		}

		log.Printf("FindMatches result: %+v", result)
		
		// Extract just the positions value
		for key, doc := range result {
			log.Printf("Found document with key %s", key)
			if docMap, ok := doc.(bson.M); ok {
				log.Printf("Document is a map with keys: %v", getMapKeys(docMap))
				if data, ok := docMap["data"].(bson.M); ok {
					log.Printf("Document has data field with keys: %v", getMapKeys(data))
					if value, ok := data[cleanPath]; ok {
						log.Printf("Found value for path %s", cleanPath)
						return []query.MatchResult{
							{
								Path:  path, // Use the original path here
								Value: value,
							},
						}, nil
					} else {
						// Try deeper nested paths
						nestedKeys := strings.Split(cleanPath, ".")
						currentMap := data
						var currentValue interface{} = nil
						found := true
						
						for i, key := range nestedKeys {
							log.Printf("Looking for nested key %s at level %d", key, i)
							if i == len(nestedKeys)-1 {
								// Last key, should be the value we want
								if val, exists := currentMap[key]; exists {
									currentValue = val
									log.Printf("Found final nested value at key %s", key)
								} else {
									found = false
									log.Printf("Final key %s not found", key)
									break
								}
							} else {
								// Not the last key, should be another map
								if nextMap, exists := currentMap[key].(bson.M); exists {
									currentMap = nextMap
									log.Printf("Found nested map at key %s with keys: %v", key, getMapKeys(nextMap))
								} else {
									found = false
									log.Printf("Nested key %s not found or not a map", key)
									break
								}
							}
						}
						
						if found && currentValue != nil {
							log.Printf("Found value through nested path traversal")
							return []query.MatchResult{
								{
									Path:  path, // Use the original path here
									Value: currentValue,
								},
							}, nil
						}
					}
				}
			}
		}

		log.Printf("No matches found after processing document")
		return []query.MatchResult{}, nil
	} else {
		// Document mode - original implementation
		// Get the document
		var doc Document
		err := s.collection.FindOne(ctx, bson.M{"_id": s.documentID}).Decode(&doc)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				// If document doesn't exist, return an empty result
				return []query.MatchResult{}, nil
			}
			return nil, err
		}

		// Create a matcher
		matcher := query.NewMatcher()
		
		// Find matches
		return matcher.Match(doc.Data, path)
	}
}

// Helper function to get map keys for logging
func getMapKeys(m bson.M) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Disconnect closes the MongoDB connection when the store is no longer needed
func (s *MongoStore) Disconnect() error {
	// Cancel the background context
	s.cancelFunc()
	
	// Create a context with timeout for disconnection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	// Disconnect from MongoDB
	return s.client.Disconnect(ctx)
}

// DisplayStoreInfo lists all databases, collections, and documents at startup for debugging
func (s *MongoStore) DisplayStoreInfo() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Display connected MongoDB server info
	serverStatus, err := s.database.RunCommand(ctx, bson.D{{"serverStatus", 1}}).DecodeBytes()
	if err != nil {
		log.Printf("Error getting server status: %v", err)
	} else {
		version, err := serverStatus.LookupErr("version")
		if err == nil {
			log.Printf("Connected to MongoDB server version: %s", version.StringValue())
		}
		
		host, err := serverStatus.LookupErr("host")
		if err == nil {
			log.Printf("MongoDB server host: %s", host.StringValue())
		}
	}

	log.Println("====== MongoDB Information ======")
	
	// Report the mode
	if s.useCollection {
		log.Println("Store Mode: Collection is root (each document in collection is root level)")
	} else {
		log.Printf("Store Mode: Document is root (ID: %s)", s.documentID)
	}

	// List databases
	databases, err := s.client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("failed to list databases: %w", err)
	}

	// Count total collections and documents across all databases
	totalCollections := 0
	totalDocuments := 0
	dbCollectionMap := make(map[string][]string)
	dbDocumentCountMap := make(map[string]int64)

	// Loop through databases to gather statistics
	log.Printf("Found %d databases:", len(databases))
	for _, dbName := range databases {
		db := s.client.Database(dbName)
		
		// List collections in this database
		collections, err := db.ListCollectionNames(ctx, bson.M{})
		if err != nil {
			log.Printf("  Database: %s (error listing collections: %v)", dbName, err)
			continue
		}

		// Store collections for this database
		dbCollectionMap[dbName] = collections
		totalCollections += len(collections)
		
		log.Printf("  Database: %s (%d collections)", dbName, len(collections))
		
		// Count documents in each collection
		var dbDocCount int64 = 0
		for _, collName := range collections {
			coll := db.Collection(collName)
			
			// Count documents
			count, err := coll.CountDocuments(ctx, bson.M{})
			if err != nil {
				log.Printf("    Collection: %s (error counting documents: %v)", collName, err)
				continue
			}
			
			dbDocCount += count
			totalDocuments += int(count)
			
			log.Printf("    Collection: %s (%d documents)", collName, count)
		}
		
		// Store total document count for this database
		dbDocumentCountMap[dbName] = dbDocCount
		
		// Only show detailed information for the db we're using
		if dbName == s.database.Name() {
			log.Printf("  > Current database: %s (total documents: %d)", dbName, dbDocCount)
			
			for _, collName := range collections {
				coll := db.Collection(collName)
				
				// Count documents
				count, err := coll.CountDocuments(ctx, bson.M{})
				if err != nil {
					continue
				}
				
				// Only show details for our collection
				if collName == s.collection.Name() {
					log.Printf("    > Current collection: %s (%d documents)", collName, count)
					
					// List documents (limit to first 10)
					cursor, err := coll.Find(ctx, bson.M{}, options.Find().SetLimit(10))
					if err != nil {
						log.Printf("      Error listing documents: %v", err)
						continue
					}
					defer cursor.Close(ctx)
					
					// If in collection mode, display documents differently
					if s.useCollection {
						// Display root documents
						var documents []bson.M
						if err := cursor.All(ctx, &documents); err != nil {
							log.Printf("      Error decoding documents: %v", err)
							continue
						}
						
						log.Printf("      Documents in %s (showing up to 10):", collName)
						for i, doc := range documents {
							var id interface{} = "unknown"
							if val, ok := doc["_id"]; ok {
								id = val
							}
							
							// Convert document data to JSON for display
							jsonData, err := json.MarshalIndent(doc, "        ", "  ")
							if err != nil {
								log.Printf("        Document %d (ID: %v) (error marshaling: %v)", i+1, id, err)
								continue
							}
							
							jsonStr := string(jsonData)
							// Truncate if too long
							if len(jsonStr) > 500 {
								jsonStr = jsonStr[:500] + "... (truncated)"
							}
							
							// Count fields
							fieldCount := len(doc)
							
							log.Printf("        Document %d (ID: %v, Fields: %d): %s", 
								i+1, id, fieldCount, jsonStr)
						}
						
						if count > 10 {
							log.Printf("        ... and %d more documents", count-10)
						}
					} else {
						// Original document mode display
						var documents []Document
						if err := cursor.All(ctx, &documents); err != nil {
							log.Printf("      Error decoding documents: %v", err)
							continue
						}
						
						log.Printf("      Documents in %s (showing up to 10):", collName)
						for i, doc := range documents {
							// Convert document data to JSON for display
							jsonData, err := json.MarshalIndent(doc, "        ", "  ")
							if err != nil {
								log.Printf("        Document %d (ID: %s) (error marshaling: %v)", i+1, doc.ID, err)
								continue
							}
							
							jsonStr := string(jsonData)
							// Truncate if too long
							if len(jsonStr) > 500 {
								jsonStr = jsonStr[:500] + "... (truncated)"
							}
							
							log.Printf("        Document %d (ID: %s): %s", i+1, doc.ID, jsonStr)
						}
						
						if count > 10 {
							log.Printf("        ... and %d more documents", count-10)
						}
					}
				} else {
					// For other collections, show sample first document
					if count > 0 {
						cursor, err := coll.Find(ctx, bson.M{}, options.Find().SetLimit(1))
						if err == nil {
							defer cursor.Close(ctx)
							var doc bson.M
							if cursor.Next(ctx) {
								if err := cursor.Decode(&doc); err == nil {
									// Convert to JSON for display
									jsonData, err := json.Marshal(doc)
									if err == nil {
										jsonStr := string(jsonData)
										if len(jsonStr) > 200 {
											jsonStr = jsonStr[:200] + "... (truncated)"
										}
										log.Printf("      Sample document: %s", jsonStr)
									}
								}
							}
						}
					}
				}
			}
		}
	}
	
	// Print collection statistics summary
	log.Println("\n===== MongoDB Statistics Summary =====")
	log.Printf("Total Databases: %d", len(databases))
	log.Printf("Total Collections: %d", totalCollections)
	log.Printf("Total Documents: %d", totalDocuments)
	log.Printf("Current Database: %s", s.database.Name())
	log.Printf("Current Collection: %s", s.collection.Name())
	
	// Display info based on mode
	if s.useCollection {
		// Collection mode - get collection summary
		count, err := s.collection.CountDocuments(ctx, bson.M{})
		if err == nil {
			log.Printf("Current Collection Document Count: %d", count)
		}
		
		// Get stats about document sizes (sample a few documents)
		cursor, err := s.collection.Find(ctx, bson.M{}, options.Find().SetLimit(5))
		if err == nil {
			defer cursor.Close(ctx)
			var totalSize int
			var docsCount int
			
			for cursor.Next(ctx) {
				var doc bson.M
				if err := cursor.Decode(&doc); err == nil {
					jsonData, err := json.Marshal(doc)
					if err == nil {
						totalSize += len(jsonData)
						docsCount++
					}
				}
			}
			
			if docsCount > 0 {
				avgSize := float64(totalSize) / float64(docsCount)
				log.Printf("Average Document Size (from sample): %.2f KB", avgSize/1024.0)
			}
		}
	} else {
		// Document mode - get specific document
		var doc Document
		err = s.collection.FindOne(ctx, bson.M{"_id": s.documentID}).Decode(&doc)
		if err == nil {
			// Get the size of the data
			jsonData, err := json.Marshal(doc.Data)
			if err == nil {
				dataSizeKB := float64(len(jsonData)) / 1024.0
				log.Printf("Current Document (ID: %s) Size: %.2f KB", s.documentID, dataSizeKB)
				
				// Count top-level keys
				if doc.Data != nil {
					log.Printf("Current Document Top-level Keys: %d", len(doc.Data))
				}
			}
		} else if err == mongo.ErrNoDocuments {
			log.Printf("Current Document (ID: %s) does not exist yet", s.documentID)
		} else {
			log.Printf("Error retrieving current document: %v", err)
		}
	}
	
	log.Println("================================")
	return nil
}

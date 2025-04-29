package store_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/piske-alex/go-sse/internal/store"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// skipIfNoMongo skips tests if MongoDB is not available
func skipIfNoMongo(t *testing.T) {
	// Check if MongoDB URI is set
	mongouri := os.Getenv("MONGO_URI")
	if mongouri == "" {
		mongouri = "mongodb://localhost:27017"
	}

	// Try to connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(mongouri))
	if err != nil {
		t.Skip("MongoDB not available, skipping test")
	}

	// Ping MongoDB
	err = client.Ping(ctx, nil)
	if err != nil {
		// Disconnect if ping fails
		client.Disconnect(ctx)
		t.Skip("MongoDB not available, skipping test")
	}

	// Disconnect
	client.Disconnect(ctx)
}

func TestMongoStore_Initialize(t *testing.T) {
	skipIfNoMongo(t)

	// Create a test MongoDB store
	mongouri := os.Getenv("MONGO_URI")
	if mongouri == "" {
		mongouri = "mongodb://localhost:27017"
	}

	// Use a test database and collection
	dbName := "gosse_test"
	collectionName := "store_test"
	documentID := "test_" + time.Now().Format("20060102150405")

	// Create the store
	store, err := store.NewMongoStore(mongouri, dbName, collectionName, documentID)
	if err != nil {
		t.Fatalf("Failed to create MongoDB store: %v", err)
	}
	defer store.Disconnect()

	// Test data
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":     1,
				"name":   "Alice",
				"status": "online",
			},
			map[string]interface{}{
				"id":     2,
				"name":   "Bob",
				"status": "offline",
			},
		},
		"config": map[string]interface{}{
			"maxUsers": 100,
			"timeout":  30,
		},
	}

	// Initialize the store
	err = store.Initialize(data)
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Get the data back
	result, err := store.Get("")
	if err != nil {
		t.Fatalf("Failed to get data: %v", err)
	}

	// Verify the result
	resultData, ok := result.(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", result)
	}

	// Check if users exists
	users, ok := resultData["users"]
	if !ok {
		t.Fatalf("users key not found in result")
	}

	// Check if users is a slice
	usersList, ok := users.([]interface{})
	if !ok {
		t.Fatalf("Expected users to be []interface{}, got %T", users)
	}

	// Check if we have the expected number of users
	if len(usersList) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(usersList))
	}
}

func TestMongoStore_SetAndGet(t *testing.T) {
	skipIfNoMongo(t)

	// Create a test MongoDB store
	mongouri := os.Getenv("MONGO_URI")
	if mongouri == "" {
		mongouri = "mongodb://localhost:27017"
	}

	// Use a test database and collection
	dbName := "gosse_test"
	collectionName := "store_test"
	documentID := "test_" + time.Now().Format("20060102150405")

	// Create the store
	store, err := store.NewMongoStore(mongouri, dbName, collectionName, documentID)
	if err != nil {
		t.Fatalf("Failed to create MongoDB store: %v", err)
	}
	defer store.Disconnect()

	// Initialize with empty data
	err = store.Initialize(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Set a value
	path := ".users[0].status"
	value := "online"
	err = store.Set(path, value)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get the value back
	result, err := store.Get(path)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	// Verify the result
	if result != value {
		t.Fatalf("Expected %v, got %v", value, result)
	}

	// Set another value
	path = ".config.maxUsers"
	value = 100
	err = store.Set(path, value)
	if err != nil {
		t.Fatalf("Failed to set value: %v", err)
	}

	// Get the value back
	result, err = store.Get(path)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	// Verify the result
	if result != float64(100) { // MongoDB returns numbers as float64
		t.Fatalf("Expected %v, got %v", float64(100), result)
	}
}

func TestMongoStore_SetFromJSON(t *testing.T) {
	skipIfNoMongo(t)

	// Create a test MongoDB store
	mongouri := os.Getenv("MONGO_URI")
	if mongouri == "" {
		mongouri = "mongodb://localhost:27017"
	}

	// Use a test database and collection
	dbName := "gosse_test"
	collectionName := "store_test"
	documentID := "test_" + time.Now().Format("20060102150405")

	// Create the store
	store, err := store.NewMongoStore(mongouri, dbName, collectionName, documentID)
	if err != nil {
		t.Fatalf("Failed to create MongoDB store: %v", err)
	}
	defer store.Disconnect()

	// Initialize with empty data
	err = store.Initialize(map[string]interface{}{})
	if err != nil {
		t.Fatalf("Failed to initialize store: %v", err)
	}

	// Set a value from JSON
	path := ".users"
	jsonData := `[
		{"id": 1, "name": "Alice", "status": "online"},
		{"id": 2, "name": "Bob", "status": "offline"}
	]`

	err = store.SetFromJSON(path, []byte(jsonData))
	if err != nil {
		t.Fatalf("Failed to set value from JSON: %v", err)
	}

	// Get the value back
	result, err := store.Get(path)
	if err != nil {
		t.Fatalf("Failed to get value: %v", err)
	}

	// Verify the result
	usersList, ok := result.([]interface{})
	if !ok {
		t.Fatalf("Expected []interface{}, got %T", result)
	}

	// Check if we have the expected number of users
	if len(usersList) != 2 {
		t.Fatalf("Expected 2 users, got %d", len(usersList))
	}

	// Check if the first user has the expected name
	user1, ok := usersList[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected map[string]interface{}, got %T", usersList[0])
	}

	if user1["name"] != "Alice" {
		t.Fatalf("Expected name to be 'Alice', got %v", user1["name"])
	}
}

func TestMongoStore_LargeDocument(t *testing.T) {
	skipIfNoMongo(t)

	// Create a test MongoDB store
	mongouri := os.Getenv("MONGO_URI")
	if mongouri == "" {
		mongouri = "mongodb://localhost:27017"
	}

	// Use a test database and collection
	dbName := "gosse_test"
	collectionName := "store_test"
	documentID := "test_large_" + time.Now().Format("20060102150405")

	// Create the store
	store, err := store.NewMongoStore(mongouri, dbName, collectionName, documentID)
	if err != nil {
		t.Fatalf("Failed to create MongoDB store: %v", err)
	}
	defer store.Disconnect()

	// Generate a large nested document (approximately 200KB)
	users := make([]interface{}, 1000)
	for i := 0; i < 1000; i++ {
		users[i] = map[string]interface{}{
			"id":     i,
			"name":   "User" + string(rune(i)),
			"email":  "user" + string(rune(i)) + "@example.com",
			"status": "active",
			"profile": map[string]interface{}{
				"bio":       "This is a bio for user " + string(rune(i)),
				"location":  "City " + string(rune(i%50)),
				"interests": []string{"interest1", "interest2", "interest3"},
			},
		}
	}

	largeData := map[string]interface{}{
		"users":     users,
		"metadata":  map[string]interface{}{"lastUpdated": time.Now().String()},
		"settings":  map[string]interface{}{"theme": "dark", "notifications": true},
		"statistics": map[string]interface{}{"activeUsers": 750, "totalMessages": 15000},
	}

	// Initialize with large data
	start := time.Now()
	err = store.Initialize(largeData)
	if err != nil {
		t.Fatalf("Failed to initialize store with large data: %v", err)
	}
	elapsed := time.Since(start)
	t.Logf("Initialize took %s", elapsed)

	// Get a specific path
	start = time.Now()
	_, err = store.Get(".users[500].name")
	if err != nil {
		t.Fatalf("Failed to get specific path: %v", err)
	}
	elapsed = time.Since(start)
	t.Logf("Get specific path took %s", elapsed)

	// Update a specific path
	start = time.Now()
	err = store.Set(".users[750].status", "away")
	if err != nil {
		t.Fatalf("Failed to update specific path: %v", err)
	}
	elapsed = time.Since(start)
	t.Logf("Update specific path took %s", elapsed)

	// Verify the update
	result, err := store.Get(".users[750].status")
	if err != nil {
		t.Fatalf("Failed to get updated value: %v", err)
	}

	// Verify the result
	if result != "away" {
		t.Fatalf("Expected status to be 'away', got %v", result)
	}

	// Convert the large data to JSON to check its size
	jsonData, err := json.Marshal(largeData)
	if err != nil {
		t.Fatalf("Failed to marshal large data: %v", err)
	}

	t.Logf("Large document size: %d bytes (%d KB)", len(jsonData), len(jsonData)/1024)
}

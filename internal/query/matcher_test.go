package query_test

import (
	"reflect"
	"testing"

	"github.com/piske-alex/go-sse/internal/query"
)

func TestMatcher_Get(t *testing.T) {
	matcher := query.NewMatcher()

	// Test data
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":     float64(1),
				"name":   "Alice",
				"status": "online",
			},
			map[string]interface{}{
				"id":     float64(2),
				"name":   "Bob",
				"status": "offline",
			},
		},
		"config": map[string]interface{}{
			"maxUsers": float64(100),
			"timeout":  float64(30),
		},
	}

	tests := []struct {
		name     string
		path     string
		expected interface{}
		isError  bool
	}{
		{
			name:     "root path",
			path:     ".",
			expected: data,
			isError:  false,
		},
		{
			name:     "property path",
			path:     ".users",
			expected: data["users"],
			isError:  false,
		},
		{
			name:     "index path",
			path:     ".users[0]",
			expected: data["users"].([]interface{})[0],
			isError:  false,
		},
		{
			name:     "nested property path",
			path:     ".users[0].name",
			expected: "Alice",
			isError:  false,
		},
		{
			name:     "config property path",
			path:     ".config.maxUsers",
			expected: float64(100),
			isError:  false,
		},
		{
			name:     "non-existent property",
			path:     ".missing",
			expected: nil,
			isError:  true,
		},
		{
			name:     "out of bounds index",
			path:     ".users[5]",
			expected: nil,
			isError:  true,
		},
		{
			name:     "invalid path",
			path:     "users[0",
			expected: nil,
			isError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := matcher.Get(data, tt.path)

			// Check error
			if tt.isError && err == nil {
				t.Fatalf("Expected error, got nil")
			}
			if !tt.isError && err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			// Skip further checks if we expected an error
			if tt.isError {
				return
			}

			// Check result
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestMatcher_Match(t *testing.T) {
	matcher := query.NewMatcher()

	// Test data
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":     float64(1),
				"name":   "Alice",
				"status": "online",
			},
			map[string]interface{}{
				"id":     float64(2),
				"name":   "Bob",
				"status": "offline",
			},
		},
	}

	tests := []struct {
		name          string
		path          string
		expectedCount int
		isError       bool
	}{
		{
			name:          "wildcard path",
			path:          ".users[*]",
			expectedCount: 2,
			isError:       false,
		},
		{
			name:          "nested wildcard path",
			path:          ".users[*].status",
			expectedCount: 2,
			isError:       false,
		},
		{
			name:          "non-existent property",
			path:          ".missing[*]",
			expectedCount: 0,
			isError:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := matcher.Match(data, tt.path)

			// Check error
			if tt.isError && err == nil {
				t.Fatalf("Expected error, got nil")
			}
			if !tt.isError && err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			// Skip further checks if we expected an error
			if tt.isError {
				return
			}

			// Check result count
			if len(results) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(results))
			}
		})
	}
}

func TestMatcher_Set(t *testing.T) {
	matcher := query.NewMatcher()

	// Test data
	data := map[string]interface{}{
		"users": []interface{}{
			map[string]interface{}{
				"id":     float64(1),
				"name":   "Alice",
				"status": "online",
			},
		},
		"config": map[string]interface{}{
			"maxUsers": float64(100),
		},
	}

	tests := []struct {
		name     string
		path     string
		value    interface{}
		isError  bool
	}{
		{
			name:     "update property",
			path:     ".users[0].status",
			value:    "away",
			isError:  false,
		},
		{
			name:     "update config",
			path:     ".config.maxUsers",
			value:    float64(200),
			isError:  false,
		},
		{
			name:     "add new property",
			path:     ".config.timeout",
			value:    float64(30),
			isError:  false,
		},
		{
			name:     "non-existent property",
			path:     ".missing.field",
			value:    "value",
			isError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clone the data for this test
			testData := map[string]interface{}{
				"users": []interface{}{
					map[string]interface{}{
						"id":     float64(1),
						"name":   "Alice",
						"status": "online",
					},
				},
				"config": map[string]interface{}{
					"maxUsers": float64(100),
				},
			}

			err := matcher.Set(testData, tt.path, tt.value)

			// Check error
			if tt.isError && err == nil {
				t.Fatalf("Expected error, got nil")
			}
			if !tt.isError && err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			// Skip further checks if we expected an error
			if tt.isError {
				return
			}

			// Verify the value was set
			result, err := matcher.Get(testData, tt.path)
			if err != nil {
				t.Fatalf("Failed to get value after set: %v", err)
			}

			if !reflect.DeepEqual(result, tt.value) {
				t.Errorf("Expected %v, got %v", tt.value, result)
			}
		})
	}
}

package query_test

import (
	"testing"

	"github.com/piske-alex/go-sse/internal/query"
)

func TestParser_Parse(t *testing.T) {
	parser := query.NewParser()

	tests := []struct {
		name     string
		path     string
		expected []query.PathSegment
		isError  bool
	}{
		{
			name: "empty path",
			path: "",
			expected: []query.PathSegment{
				{Type: query.Root, Value: "", Index: -1},
			},
			isError: false,
		},
		{
			name: "root path",
			path: ".",
			expected: []query.PathSegment{
				{Type: query.Root, Value: "", Index: -1},
			},
			isError: false,
		},
		{
			name: "property path",
			path: ".users",
			expected: []query.PathSegment{
				{Type: query.Root, Value: "", Index: -1},
				{Type: query.Property, Value: "users", Index: -1},
			},
			isError: false,
		},
		{
			name: "index path",
			path: ".users[0]",
			expected: []query.PathSegment{
				{Type: query.Root, Value: "", Index: -1},
				{Type: query.Property, Value: "users", Index: -1},
				{Type: query.Index, Value: "", Index: 0},
			},
			isError: false,
		},
		{
			name: "wildcard path",
			path: ".users[*]",
			expected: []query.PathSegment{
				{Type: query.Root, Value: "", Index: -1},
				{Type: query.Property, Value: "users", Index: -1},
				{Type: query.Wildcard, Value: "", Index: -1},
			},
			isError: false,
		},
		{
			name: "complex path",
			path: ".users[0].name",
			expected: []query.PathSegment{
				{Type: query.Root, Value: "", Index: -1},
				{Type: query.Property, Value: "users", Index: -1},
				{Type: query.Index, Value: "", Index: 0},
				{Type: query.Property, Value: "name", Index: -1},
			},
			isError: false,
		},
		{
			name:     "invalid path",
			path:     "users[0].name", // Missing leading dot
			expected: nil,
			isError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := parser.Parse(tt.path)

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

			// Check segments count
			if len(segments) != len(tt.expected) {
				t.Fatalf("Expected %d segments, got %d", len(tt.expected), len(segments))
			}

			// Check each segment
			for i, expectedSegment := range tt.expected {
				if segments[i].Type != expectedSegment.Type {
					t.Errorf("Segment %d: expected type %v, got %v", i, expectedSegment.Type, segments[i].Type)
				}
				if segments[i].Value != expectedSegment.Value {
					t.Errorf("Segment %d: expected value %q, got %q", i, expectedSegment.Value, segments[i].Value)
				}
				if segments[i].Index != expectedSegment.Index {
					t.Errorf("Segment %d: expected index %d, got %d", i, expectedSegment.Index, segments[i].Index)
				}
			}
		})
	}
}

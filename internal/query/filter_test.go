package query_test

import (
	"testing"

	"github.com/piske-alex/go-sse/internal/query"
)

func TestFilter_IsMatch(t *testing.T) {
	tests := []struct {
		name          string
		filterPath    string
		changePath    string
		changeValue   interface{}
		shouldMatch   bool
	}{
		{
			name:          "exact match",
			filterPath:    ".users[0].status",
			changePath:    ".users[0].status",
			changeValue:   "away",
			shouldMatch:   true,
		},
		{
			name:          "parent path match",
			filterPath:    ".users[0].status",
			changePath:    ".users[0]",
			changeValue:   map[string]interface{}{"status": "away"},
			shouldMatch:   true,
		},
		{
			name:          "child path match",
			filterPath:    ".users",
			changePath:    ".users[0].status",
			changeValue:   "away",
			shouldMatch:   true,
		},
		{
			name:          "root match",
			filterPath:    ".",
			changePath:    ".users[0].status",
			changeValue:   "away",
			shouldMatch:   true,
		},
		{
			name:          "wildcard match",
			filterPath:    ".users[*].status",
			changePath:    ".users[0].status",
			changeValue:   "away",
			shouldMatch:   true,
		},
		{
			name:          "no match",
			filterPath:    ".config.timeout",
			changePath:    ".users[0].status",
			changeValue:   "away",
			shouldMatch:   false,
		},
		{
			name:          "sibling no match",
			filterPath:    ".users[1].status",
			changePath:    ".users[0].status",
			changeValue:   "away",
			shouldMatch:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := query.NewFilter(tt.filterPath)
			
			match := filter.IsMatch(tt.changePath, tt.changeValue)
			
			if match != tt.shouldMatch {
				t.Errorf("Expected match to be %v, got %v for filter '%s' and change '%s'", 
					tt.shouldMatch, match, tt.filterPath, tt.changePath)
			}
		})
	}
}

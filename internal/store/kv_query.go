package store

import (
	"github.com/piske-alex/go-sse/internal/query"
)

// Integration of KV store with query package

// getValueByPath retrieves a value by path using the query package
func (s *Store) getValueByPath(data map[string]interface{}, path string) (interface{}, error) {
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Get the value at the path
	result, err := matcher.Get(data, path)
	if err != nil {
		if err == query.ErrPathNotFound {
			return nil, ErrPathNotFound
		}
		return nil, err
	}
	
	return result, nil
}

// setValueByPath sets a value at the specified path using the query package
func (s *Store) setValueByPath(data map[string]interface{}, path string, value interface{}) error {
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Set the value at the path
	err := matcher.Set(data, path, value)
	if err != nil {
		if err == query.ErrPathNotFound {
			return ErrPathNotFound
		}
		return err
	}
	
	return nil
}

// deleteByPath deletes a value at the specified path using the query package
func (s *Store) deleteByPath(data map[string]interface{}, path string) error {
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Delete the value at the path
	err := matcher.Delete(data, path)
	if err != nil {
		if err == query.ErrPathNotFound {
			return ErrPathNotFound
		}
		return err
	}
	
	return nil
}

// FindMatches finds all values matching a path expression
func (s *Store) FindMatches(path string) ([]query.MatchResult, error) {
	s.mux.RLock()
	defer s.mux.RUnlock()
	
	// Create a matcher
	matcher := query.NewMatcher()
	
	// Find matches
	results, err := matcher.Match(s.data, path)
	if err != nil {
		return nil, err
	}
	
	return results, nil
}

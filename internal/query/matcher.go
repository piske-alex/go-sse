package query

import (
	"errors"
)

// ErrInvalidPath indicates an invalid path expression
var ErrInvalidPath = errors.New("invalid path expression")

// ErrPathNotFound indicates a path doesn't exist in the data
var ErrPathNotFound = errors.New("path not found in data")

// MatchResult represents a match result with path and value
type MatchResult struct {
	Path  string
	Value interface{}
}

// Matcher handles matching of paths against data
type Matcher struct {
	parser *Parser
}

// NewMatcher creates a new path matcher
func NewMatcher() *Matcher {
	return &Matcher{
		parser: NewParser(),
	}
}

// Get retrieves a value from data using the path expression
func (m *Matcher) Get(data interface{}, path string) (interface{}, error) {
	// Parse the path
	segments, err := m.parser.Parse(path)
	if err != nil {
		return nil, err
	}

	// Navigate through the data using the segments
	result, err := m.navigateSegments(data, segments, 1) // Start from index 1 to skip root
	if err != nil {
		return nil, err
	}

	return result, nil
}

// Match finds all values matching the path expression
func (m *Matcher) Match(data interface{}, path string) ([]MatchResult, error) {
	// Parse the path
	segments, err := m.parser.Parse(path)
	if err != nil {
		return nil, err
	}

	// Match segments against data
	var results []MatchResult
	err = m.matchSegments(data, segments, 1, "", &results) // Start from index 1 to skip root
	if err != nil {
		return nil, err
	}

	return results, nil
}

// Set updates a value in the data using the path expression
func (m *Matcher) Set(data interface{}, path string, value interface{}) error {
	// Parse the path
	segments, err := m.parser.Parse(path)
	if err != nil {
		return err
	}

	// Set the value using the segments
	return m.setValueBySegments(data, segments, 1, value) // Start from index 1 to skip root
}

// Delete removes a value from the data using the path expression
func (m *Matcher) Delete(data interface{}, path string) error {
	// Parse the path
	segments, err := m.parser.Parse(path)
	if err != nil {
		return err
	}

	// Delete the value using the segments
	return m.deleteBySegments(data, segments, 1) // Start from index 1 to skip root
}

// navigateSegments navigates through the data using segments
func (m *Matcher) navigateSegments(data interface{}, segments []PathSegment, index int) (interface{}, error) {
	// Return data if we've processed all segments
	if index >= len(segments) {
		return data, nil
	}

	segment := segments[index]

	switch segment.Type {
	case Property:
		// Handle a property segment
		map_data, ok := data.(map[string]interface{})
		if !ok {
			return nil, ErrPathNotFound
		}

		value, exists := map_data[segment.Value]
		if !exists {
			return nil, ErrPathNotFound
		}

		return m.navigateSegments(value, segments, index+1)

	case Index:
		// Handle an array index segment
		slice_data, ok := data.([]interface{})
		if !ok {
			return nil, ErrPathNotFound
		}

		if segment.Index < 0 || segment.Index >= len(slice_data) {
			return nil, ErrPathNotFound
		}

		return m.navigateSegments(slice_data[segment.Index], segments, index+1)

	case Wildcard:
		// Wildcards not supported in Get - use Match instead
		return nil, errors.New("wildcards not supported in Get operation")

	default:
		return nil, ErrInvalidPath
	}
}

// matchSegments finds all values matching the segments
func (m *Matcher) matchSegments(data interface{}, segments []PathSegment, index int, currentPath string, results *[]MatchResult) error {
	// If we've processed all segments, add the result
	if index >= len(segments) {
		*results = append(*results, MatchResult{Path: currentPath, Value: data})
		return nil
	}

	segment := segments[index]
	switch segment.Type {
	case Property:
		// Handle a property segment
		mapData, ok := data.(map[string]interface{})
		if !ok {
			return nil
		}

		value, exists := mapData[segment.Value]
		if !exists {
			return nil
		}

		newPath := currentPath + "." + segment.Value
		return m.matchSegments(value, segments, index+1, newPath, results)

	case Index:
		// Handle an array index segment
		sliceData, ok := data.([]interface{})
		if !ok {
			return nil
		}

		if segment.Index < 0 || segment.Index >= len(sliceData) {
			return nil
		}

		newPath := currentPath + "[" + string(segment.Index) + "]"
		return m.matchSegments(sliceData[segment.Index], segments, index+1, newPath, results)

	case Wildcard:
		// Handle a wildcard segment
		sliceData, ok := data.([]interface{})
		if !ok {
			return nil
		}

		for i, item := range sliceData {
			newPath := currentPath + "[" + string(i) + "]"
			err := m.matchSegments(item, segments, index+1, newPath, results)
			if err != nil {
				// Continue despite errors in wildcard matching
				continue
			}
		}

		return nil

	default:
		return ErrInvalidPath
	}
}

// setValueBySegments updates a value in the data using segments
func (m *Matcher) setValueBySegments(data interface{}, segments []PathSegment, index int, value interface{}) error {
	// If we've processed all but the last segment, set the value
	if index == len(segments)-1 {
		return m.setFinalSegment(data, segments[index], value)
	}

	segment := segments[index]

	switch segment.Type {
	case Property:
		// Handle a property segment
		mapData, ok := data.(map[string]interface{})
		if !ok {
			return ErrPathNotFound
		}

		value, exists := mapData[segment.Value]
		if !exists {
			// Create missing intermediate objects
			if segments[index+1].Type == Property {
				mapData[segment.Value] = make(map[string]interface{})
			} else if segments[index+1].Type == Index {
				mapData[segment.Value] = make([]interface{}, segments[index+1].Index+1)
			} else {
				return ErrPathNotFound
			}
			value = mapData[segment.Value]
		}

		return m.setValueBySegments(value, segments, index+1, value)

	case Index:
		// Handle an array index segment
		sliceData, ok := data.([]interface{})
		if !ok {
			return ErrPathNotFound
		}

		if segment.Index < 0 || segment.Index >= len(sliceData) {
			return ErrPathNotFound
		}

		return m.setValueBySegments(sliceData[segment.Index], segments, index+1, value)

	case Wildcard:
		// Wildcards not supported in Set
		return errors.New("wildcards not supported in Set operation")

	default:
		return ErrInvalidPath
	}
}

// setFinalSegment sets the value for the final segment
func (m *Matcher) setFinalSegment(data interface{}, segment PathSegment, value interface{}) error {
	switch segment.Type {
	case Property:
		// Set a property value
		mapData, ok := data.(map[string]interface{})
		if !ok {
			return ErrPathNotFound
		}

		mapData[segment.Value] = value
		return nil

	case Index:
		// Set an array index value
		sliceData, ok := data.([]interface{})
		if !ok {
			return ErrPathNotFound
		}

		if segment.Index < 0 || segment.Index >= len(sliceData) {
			return ErrPathNotFound
		}

		sliceData[segment.Index] = value
		return nil

	case Wildcard:
		// Wildcards not supported in Set
		return errors.New("wildcards not supported in Set operation")

	default:
		return ErrInvalidPath
	}
}

// deleteBySegments removes a value from the data using segments
func (m *Matcher) deleteBySegments(data interface{}, segments []PathSegment, index int) error {
	// If we've processed all but the last segment, delete the value
	if index == len(segments)-1 {
		return m.deleteFinalSegment(data, segments[index])
	}

	segment := segments[index]

	switch segment.Type {
	case Property:
		// Handle a property segment
		mapData, ok := data.(map[string]interface{})
		if !ok {
			return ErrPathNotFound
		}

		value, exists := mapData[segment.Value]
		if !exists {
			return ErrPathNotFound
		}

		return m.deleteBySegments(value, segments, index+1)

	case Index:
		// Handle an array index segment
		sliceData, ok := data.([]interface{})
		if !ok {
			return ErrPathNotFound
		}

		if segment.Index < 0 || segment.Index >= len(sliceData) {
			return ErrPathNotFound
		}

		return m.deleteBySegments(sliceData[segment.Index], segments, index+1)

	case Wildcard:
		// Wildcards not supported in Delete
		return errors.New("wildcards not supported in Delete operation")

	default:
		return ErrInvalidPath
	}
}

// deleteFinalSegment deletes the value for the final segment
func (m *Matcher) deleteFinalSegment(data interface{}, segment PathSegment) error {
	switch segment.Type {
	case Property:
		// Delete a property
		mapData, ok := data.(map[string]interface{})
		if !ok {
			return ErrPathNotFound
		}

		_, exists := mapData[segment.Value]
		if !exists {
			return ErrPathNotFound
		}

		delete(mapData, segment.Value)
		return nil

	case Index:
		// Delete an array index by setting to nil
		// (Go doesn't allow true removal without recreating the slice)
		sliceData, ok := data.([]interface{})
		if !ok {
			return ErrPathNotFound
		}

		if segment.Index < 0 || segment.Index >= len(sliceData) {
			return ErrPathNotFound
		}

		sliceData[segment.Index] = nil
		return nil

	case Wildcard:
		// Wildcards not supported in Delete
		return errors.New("wildcards not supported in Delete operation")

	default:
		return ErrInvalidPath
	}
}

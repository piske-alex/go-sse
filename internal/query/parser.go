package query

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// PathSegment represents a segment in a path expression
type PathSegment struct {
	Type  SegmentType
	Value string
	Index int
}

// SegmentType defines the type of path segment
type SegmentType int

const (
	// Root represents the root of the data structure
	Root SegmentType = iota
	// Property represents a map/object property
	Property
	// Index represents an array index
	Index
	// Wildcard represents a wildcard selector
	Wildcard
)

// Parser handles the parsing of JQ-style path expressions
type Parser struct {
	pathRegex        *regexp.Regexp
	propertyRegex    *regexp.Regexp
	arrayIndexRegex  *regexp.Regexp
	arraySliceRegex  *regexp.Regexp
	wildcardRegex    *regexp.Regexp
}

// NewParser creates a new path parser instance
func NewParser() *Parser {
	return &Parser{
		pathRegex:        regexp.MustCompile(`\.[^.\[\]]+|\[\d+\]|\[\*\]`),
		propertyRegex:    regexp.MustCompile(`^\.(\w+)$`),
		arrayIndexRegex:  regexp.MustCompile(`^\[(\d+)\]$`),
		wildcardRegex:    regexp.MustCompile(`^\[\*\]$`),
	}
}

// Parse parses a JQ-style path expression into segments
func (p *Parser) Parse(path string) ([]PathSegment, error) {
	// Handle empty path
	if path == "" || path == "." {
		return []PathSegment{{Type: Root, Value: "", Index: -1}}, nil
	}

	// Remove leading dot if present
	if strings.HasPrefix(path, ".") {
		path = path[1:]
	} else {
		return nil, errors.New("path must start with a dot")
	}

	// Initialize with root segment
	segments := []PathSegment{{Type: Root, Value: "", Index: -1}}

	// Find all segments using regex
	matches := p.pathRegex.FindAllString(path, -1)

	for _, match := range matches {
		if p.propertyRegex.MatchString(match) {
			// Property segment
			submatches := p.propertyRegex.FindStringSubmatch(match)
			segments = append(segments, PathSegment{
				Type:  Property,
				Value: submatches[1],
				Index: -1,
			})
		} else if p.arrayIndexRegex.MatchString(match) {
			// Array index segment
			submatches := p.arrayIndexRegex.FindStringSubmatch(match)
			index, _ := strconv.Atoi(submatches[1])
			segments = append(segments, PathSegment{
				Type:  Index,
				Value: "",
				Index: index,
			})
		} else if p.wildcardRegex.MatchString(match) {
			// Wildcard segment
			segments = append(segments, PathSegment{
				Type:  Wildcard,
				Value: "",
				Index: -1,
			})
		} else {
			return nil, errors.New("invalid path segment: " + match)
		}
	}

	return segments, nil
}

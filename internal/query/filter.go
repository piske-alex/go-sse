package query

import (
	"regexp"
	"strings"
)

// Filter represents a JQ-style path filter for subscriptions
type Filter struct {
	Path       string
	Expression string
	Matcher    *Matcher
}

// NewFilter creates a new filter from a JQ-style path expression
func NewFilter(expression string) *Filter {
	return &Filter{
		Expression: expression,
		Path:       expression,
		Matcher:    NewMatcher(),
	}
}

// IsMatch checks if a data change matches this filter
func (f *Filter) IsMatch(path string, value interface{}) bool {
	// If the changed path is a prefix of the filter path, or vice versa,
	// or if they match exactly, consider it a match
	
	// Check for exact match first
	if path == f.Path {
		return true
	}
	
	// Check if the change path is a parent of the filter path
	if strings.HasPrefix(f.Path, path+".") || strings.HasPrefix(f.Path, path+"[") {
		return true
	}
	
	// Check if the filter path is a parent of the change path
	if strings.HasPrefix(path, f.Path+".") || strings.HasPrefix(path, f.Path+"[") {
		return true
	}
	
	// Check for wildcards
	if strings.Contains(f.Path, "[*]") {
		// Convert JQ path to regex pattern
		pattern := f.pathToRegexPattern(f.Path)
		
		// Compile regex
		re, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		
		// Check if the path matches the regex pattern
		return re.MatchString(path)
	}
	
	return false
}

// pathToRegexPattern converts a JQ path to a regex pattern
func (f *Filter) pathToRegexPattern(path string) string {
	// Escape special regex characters
	pattern := regexp.QuoteMeta(path)
	
	// Replace escaped wildcards with regex patterns
	pattern = strings.ReplaceAll(pattern, "\\[\\*\\]", "\\[\\d+\\]")
	
	return "^" + pattern + "$"
}

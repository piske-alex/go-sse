package query

import (
	"regexp"
	"strings"
)

// KeyValueCondition represents a key-value condition for filtering
type KeyValueCondition struct {
	Key   string
	Value string
}

// Filter represents a JQ-style path filter for subscriptions
type Filter struct {
	Path        string
	Expression  string
	Matcher     *Matcher
	// Add support for key-value conditions
	Conditions  []KeyValueCondition
}

// NewFilter creates a new filter from a JQ-style path expression
func NewFilter(expression string) *Filter {
	filter := &Filter{
		Expression: expression,
		Path:       expression,
		Matcher:    NewMatcher(),
		Conditions: []KeyValueCondition{},
	}
	
	// Parse key-value conditions if any
	filter.parseKeyValueConditions()
	
	return filter
}

// parseKeyValueConditions extracts key-value conditions from a path expression
// Example: .data.positions[trader=abc] -> path=.data.positions, condition: {Key: "trader", Value: "abc"}
func (f *Filter) parseKeyValueConditions() {
	// Regular expression to find key-value conditions in brackets
	// Matches: [key=value]
	re := regexp.MustCompile(`\[([^=\[\]]+)=([^\[\]]+)\]`)
	
	// Find all key-value pairs in the expression
	matches := re.FindAllStringSubmatch(f.Expression, -1)
	
	if len(matches) > 0 {
		// Clean the base path by removing the conditions
		cleanPath := f.Path
		for _, match := range matches {
			// Add the condition
			f.Conditions = append(f.Conditions, KeyValueCondition{
				Key:   strings.TrimSpace(match[1]),
				Value: strings.TrimSpace(match[2]),
			})
			
			// Remove the condition from the path
			cleanPath = strings.Replace(cleanPath, match[0], "", 1)
		}
		
		// Update the path to the clean version (without conditions)
		f.Path = cleanPath
	}
}

// IsMatch checks if a data change matches this filter
func (f *Filter) IsMatch(path string, value interface{}) bool {
	// If the changed path is a prefix of the filter path, or vice versa,
	// or if they match exactly, consider it a match
	
	// Check for exact match first
	if path == f.Path {
		// If there are conditions, check them as well
		if len(f.Conditions) > 0 {
			return f.matchesConditions(value)
		}
		return true
	}
	
	// Check if the change path is a parent of the filter path
	if strings.HasPrefix(f.Path, path+".") || strings.HasPrefix(f.Path, path+"[") {
		// A parent path change could affect our filter path, but we need to check conditions
		if len(f.Conditions) > 0 {
			return f.matchesConditions(value)
		}
		return true
	}
	
	// Check if the filter path is a parent of the change path
	if strings.HasPrefix(path, f.Path+".") || strings.HasPrefix(path, f.Path+"[") {
		// Our filter is a parent of the changed path, so the change is relevant to us
		if len(f.Conditions) > 0 {
			return f.matchesConditions(value)
		}
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
		if re.MatchString(path) {
			// Path matches, now check conditions if any
			if len(f.Conditions) > 0 {
				return f.matchesConditions(value)
			}
			return true
		}
		return false
	}
	
	return false
}

// matchesConditions checks if the value matches all the key-value conditions
func (f *Filter) matchesConditions(value interface{}) bool {
	// If no conditions, it's a match
	if len(f.Conditions) == 0 {
		return true
	}
	
	// Get the actual data to check conditions against
	var dataToCheck interface{} = value
	
	// If we're filtering .data.positions but getting root data, we need to navigate to the positions
	if strings.HasPrefix(f.Path, ".data.") {
		fieldName := strings.TrimPrefix(f.Path, ".data.")
		
		// Try to get to the field we're filtering
		if valueMap, ok := value.(map[string]interface{}); ok {
			// Check if data key exists
			if data, ok := valueMap["data"].(map[string]interface{}); ok {
				// Check if the field exists
				if fieldValue, ok := data[fieldName]; ok {
					dataToCheck = fieldValue
				} else {
					// Field doesn't exist
					return false
				}
			} else {
				// No data field
				return false
			}
		} else {
			// Value is not a map
			return false
		}
	}
	
	// For array data, we need to check each item
	if arr, ok := dataToCheck.([]interface{}); ok {
		// At least one item must match all conditions
		for _, item := range arr {
			if itemMap, ok := item.(map[string]interface{}); ok {
				// Check if this item matches all conditions
				allMatch := true
				for _, condition := range f.Conditions {
					// Check if the key exists and has the expected value
					if value, exists := itemMap[condition.Key]; exists {
						// Convert value to string for comparison
						valueStr := ""
						switch v := value.(type) {
						case string:
							valueStr = v
						case int, int64, float64:
							valueStr = strings.TrimSpace(strings.ToLower(condition.Value))
							if valueStr != condition.Value {
								allMatch = false
							}
						default:
							// For other types, just check if the string representations match
							valueStr = strings.TrimSpace(strings.ToLower(condition.Value))
							if valueStr != condition.Value {
								allMatch = false
							}
						}
					} else {
						// Key doesn't exist
						allMatch = false
					}
				}
				
				if allMatch {
					return true
				}
			}
		}
		
		// No items matched all conditions
		return false
	}
	
	// For map data, check if it matches all conditions
	if mapData, ok := dataToCheck.(map[string]interface{}); ok {
		for _, condition := range f.Conditions {
			// Check if the key exists and has the expected value
			if value, exists := mapData[condition.Key]; exists {
				// Convert value to string for comparison
				valueStr := ""
				switch v := value.(type) {
				case string:
					valueStr = v
				case int, int64, float64:
					valueStr = strings.TrimSpace(strings.ToLower(condition.Value))
					if valueStr != condition.Value {
						return false
					}
				default:
					// For other types, just check if the string representations match
					valueStr = strings.TrimSpace(strings.ToLower(condition.Value))
					if valueStr != condition.Value {
						return false
					}
				}
			} else {
				// Key doesn't exist
				return false
			}
		}
		
		// All conditions matched
		return true
	}
	
	// Data is not in a format we can check
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

# Changelog

## [1.1.0] - Key-Value Filtering Feature - 2023-10-30

### Added
- Support for key-value filtering using the syntax `path[key=value]`
- Updated `query/filter.go` to parse and handle key-value conditions
- Enhanced `ShouldNotify` method in client.go to check key-value conditions
- Added `applyKeyValueFilters` and `matchesKeyValueConditions` utility functions
- Improved `BroadcastEvent` to filter array data based on key-value conditions
- Updated `AddClient` to support initial data filtering with key-value conditions
- Added detailed README documentation for the new filtering feature

### Technical Changes
- Created a new `KeyValueCondition` struct to store filter conditions
- Enhanced the `Filter` struct to include a slice of conditions
- Added regex pattern matching to extract key-value pairs from filter strings
- Implemented filtering logic for arrays and map data structures
- Added specific handling for data field paths with key-value conditions
- Improved debugging with more detailed logging of filter operations

### Examples
```
# Filter for positions where trader equals "abc"
GET /events?filter=.data.positions[trader=abc]

# Filter for offers with active status
GET /events?filter=.data.offers[status=active]

# Multiple filters with different conditions
GET /events?filter=.data.positions[trader=abc]&filter=.data.offers[status=active]
```

## [1.0.0] - Initial Release - 2023-09-15

### Added
- Initial implementation of SSE server with JQ-style path filtering
- Support for in-memory and MongoDB storage backends
- Basic HTTP API for store management
- Client filtering by path
- Connection pooling and management
- Docker and Docker Compose support 
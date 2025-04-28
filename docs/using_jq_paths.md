# Using JQ-Style Path Expressions

The go-sse server supports a simplified subset of JQ-style path expressions for navigating and filtering data. This document explains the supported syntax and provides examples.

## Basic Path Syntax

All paths must start with a dot (`.`) representing the root of the data structure:

- `.` - Refers to the entire data structure
- `.property` - Refers to a property of the root object
- `.property.nested` - Refers to a nested property
- `.array[0]` - Refers to the first element of an array
- `.property[0].nested` - Combines array indexing and property access
- `.array[*]` - Wildcard for all elements in an array

## Examples

### Getting Data

Given this data structure:

```json
{
  "data": {
    "users": [
      {"id": 1, "name": "Alice", "status": "online"},
      {"id": 2, "name": "Bob", "status": "offline"}
    ],
    "config": {
      "maxUsers": 100,
      "timeout": 30
    }
  }
}
```

Here are some example path expressions and what they would retrieve:

- `.data` - The entire data object
- `.data.users` - The array of all users
- `.data.users[0]` - The first user (Alice)
- `.data.users[0].name` - Alice's name ("Alice")
- `.data.config.maxUsers` - The maxUsers value (100)

### Using Wildcards

Wildcards are particularly useful for filtering:

- `.data.users[*].name` - Would match all user names ["Alice", "Bob"]
- `.data.users[*].status` - Would match all user statuses ["online", "offline"]

## Using Paths with SSE Filtering

When connecting to the SSE endpoint, you can provide a filter parameter to only receive updates for specific paths:

```
GET /events?filter=.data.users[*].status
```

This would subscribe you to receive events whenever any user's status changes, but not when other properties change.

You can also provide multiple filters by separating them with commas:

```
GET /events?filter=.data.users[*].status,.data.config.maxUsers
```

## Using Paths with Store Updates

When updating the store, you specify the path to update:

```
PATCH /store?path=.data.users[0].status
Content-Type: application/json

"away"
```

This would update only the status of the first user without affecting other data.

## Limitations

The current implementation supports a subset of JQ syntax with these limitations:

1. No support for filtering expressions like `.users[] | select(.age > 30)`
2. No support for array slices like `.users[1:3]`
3. No support for recursive descent `..`
4. Wildcards can only be used for array elements, not for property names

These features may be added in future versions.

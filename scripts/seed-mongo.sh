#!/bin/bash

# This script seeds the MongoDB database with initial data

echo "Seeding MongoDB with initial data..."

# Wait for MongoDB to start
until docker-compose exec mongo mongosh --eval "db.adminCommand('ping')" > /dev/null 2>&1; do
  echo "Waiting for MongoDB to start..."
  sleep 2
done

# Create a sample JSON data file
cat > /tmp/sample-data.json << EOL
{
  "data": {
    "users": [
      {"id": 1, "name": "Alice", "status": "online"},
      {"id": 2, "name": "Bob", "status": "offline"},
      {"id": 3, "name": "Charlie", "status": "away"}
    ],
    "config": {
      "maxUsers": 100,
      "timeout": 30,
      "features": {
        "chat": true,
        "notifications": true,
        "fileSharing": false
      }
    },
    "statistics": {
      "activeUsers": 2,
      "messagesPerDay": 156,
      "peakHours": [9, 14, 20]
    }
  }
}
EOL

# Send the data to the go-sse server
echo "Sending initial data to go-sse server..."
curl -X POST http://localhost:8080/store \
  -H "Content-Type: application/json" \
  -d @/tmp/sample-data.json

# Clean up
rm /tmp/sample-data.json

echo "MongoDB seeded successfully"

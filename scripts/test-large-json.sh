#!/bin/bash

# This script generates a large JSON document and tests storing it in MongoDB

echo "Generating large JSON test data..."

# Create a large JSON document with nested structure
cat > /tmp/large-test-data.json << EOL
{
  "data": {
    "users": [
EOL

# Generate 1000 users (approximately 2MB of JSON data)
for i in $(seq 1 1000); do
  # Last user doesn't get a comma
  if [ $i -eq 1000 ]; then
    comma=""
  else
    comma=","
  fi
  
  # Generate a user with random status
  status=$([ $(($RANDOM % 3)) -eq 0 ] && echo "online" || ([ $(($RANDOM % 2)) -eq 0 ] && echo "offline" || echo "away"))
  
  cat >> /tmp/large-test-data.json << EOL
      {
        "id": $i,
        "name": "User$i",
        "email": "user$i@example.com",
        "status": "$status",
        "profile": {
          "age": $(( $RANDOM % 50 + 18 )),
          "location": "City $(( $RANDOM % 100 ))",
          "interests": ["interest$(( $RANDOM % 20 ))", "interest$(( $RANDOM % 20 ))", "interest$(( $RANDOM % 20 ))"],
          "bio": "Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua. Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat."
        },
        "metadata": {
          "lastLogin": "2024-$(( $RANDOM % 12 + 1 ))-$(( $RANDOM % 28 + 1 ))T$(( $RANDOM % 24 )):$(( $RANDOM % 60 )):$(( $RANDOM % 60 ))Z",
          "registrationDate": "2023-$(( $RANDOM % 12 + 1 ))-$(( $RANDOM % 28 + 1 ))T$(( $RANDOM % 24 )):$(( $RANDOM % 60 )):$(( $RANDOM % 60 ))Z",
          "preferences": {
            "theme": "$([ $(($RANDOM % 2)) -eq 0 ] && echo "light" || echo "dark")",
            "notifications": $([ $(($RANDOM % 2)) -eq 0 ] && echo "true" || echo "false"),
            "language": "en-US"
          }
        }
      }$comma
EOL
done

# Close the users array and add some more data
cat >> /tmp/large-test-data.json << EOL
    ],
    "config": {
      "appName": "Test Application",
      "version": "1.0.0",
      "environment": "testing",
      "features": {
        "chat": true,
        "videoCall": false,
        "fileSharing": true,
        "encryption": true,
        "twoFactorAuth": true
      },
      "limits": {
        "maxUsers": 10000,
        "maxMessageLength": 2000,
        "maxFileSize": 10485760,
        "maxFilesPerUser": 100,
        "maxGroupSize": 50
      },
      "servers": [
        {
          "id": "server1",
          "name": "Primary Server",
          "location": "US-East",
          "capacity": 5000
        },
        {
          "id": "server2",
          "name": "Backup Server",
          "location": "US-West",
          "capacity": 3000
        },
        {
          "id": "server3",
          "name": "EU Server",
          "location": "EU-Central",
          "capacity": 4000
        }
      ]
    },
    "statistics": {
      "activeUsers": 423,
      "messagesPerDay": 15632,
      "averageSessionDuration": 1243,
      "peakHours": [9, 14, 20],
      "deviceDistribution": {
        "mobile": 65,
        "desktop": 30,
        "tablet": 5
      },
      "historyData": [
EOL

# Generate historical data for past 365 days
for i in $(seq 1 365); do
  # Last data point doesn't get a comma
  if [ $i -eq 365 ]; then
    comma=""
  else
    comma=","
  fi
  
  cat >> /tmp/large-test-data.json << EOL
        {
          "date": "2023-$(( ($i-1) / 30 + 1 ))-$(( ($i-1) % 30 + 1 ))",
          "activeUsers": $(( $RANDOM % 1000 + 100 )),
          "newUsers": $(( $RANDOM % 100 )),
          "messagesSent": $(( $RANDOM % 20000 + 5000 ))
        }$comma
EOL
done

# Close the JSON structure
cat >> /tmp/large-test-data.json << EOL
      ]
    }
  }
}
EOL

# Get the file size
file_size=$(du -h /tmp/large-test-data.json | cut -f1)
echo "Generated a $file_size JSON file"

# Send the data to the go-sse server
echo "Sending large data to go-sse server..."
time curl -X POST http://localhost:8080/store \
  -H "Content-Type: application/json" \
  -d @/tmp/large-test-data.json

echo "\nWaiting 2 seconds..."
sleep 2

# Test querying the data
echo "Testing query for the first user's status:"
curl -s "http://localhost:8080/store?path=.data.users[0].status"
echo "\n"

# Test updating a single field
echo "Testing update of a single field:"
time curl -X PATCH "http://localhost:8080/store?path=.data.users[0].status" \
  -H "Content-Type: application/json" \
  -d '"away"'

echo "\nTest completed successfully."

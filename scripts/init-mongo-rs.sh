#!/bin/bash

# This script initializes a MongoDB replica set for local development
# It's needed because MongoDB change streams require a replica set

echo "Initializing MongoDB replica set..."

# Wait for MongoDB to start
until docker-compose exec mongo mongosh --eval "db.adminCommand('ping')" > /dev/null 2>&1; do
  echo "Waiting for MongoDB to start..."
  sleep 2
done

# Initialize the replica set
docker-compose exec mongo mongosh --eval "
  rs.initiate({
    _id: 'rs0',
    members: [
      { _id: 0, host: 'localhost:27017' }
    ]
  })
"

# Wait for the replica set to initialize
sleep 5

# Check the replica set status
docker-compose exec mongo mongosh --eval "rs.status()"

echo "MongoDB replica set initialized successfully"
echo "You can now use change streams with MongoDB"

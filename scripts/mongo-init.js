// This script initializes the MongoDB database with the necessary user accounts and collections
// It will be executed when the MongoDB container starts for the first time

// Connect to MongoDB
print('Start MongoDB initialization script');

// Create application database and collections
db = db.getSiblingDB(process.env.MONGO_INITDB_DATABASE || 'gosse');

// Create collections
db.createCollection('kv_store');

print('Created collections in database: ' + db.getName());

// Create application user if not using root user
// Uncomment and modify this if you want to create a separate database user
/*
db.createUser({
  user: process.env.MONGO_APP_USER || 'app_user',
  pwd: process.env.MONGO_APP_PASSWORD || 'app_password',
  roles: [{ role: 'readWrite', db: db.getName() }]
});

print('Created application user for database: ' + db.getName());
*/

// Create an index on the _id field of the kv_store collection
db.kv_store.createIndex({ "_id": 1 }, { unique: true });

// Initialize with a default empty document if one doesn't exist
var mainDoc = db.kv_store.findOne({ _id: 'main' });
if (!mainDoc) {
  db.kv_store.insertOne({
    _id: 'main',
    data: {},
    created_at: new Date(),
    updated_at: new Date()
  });
  print('Created default main document in kv_store collection');
}

print('MongoDB initialization completed successfully');

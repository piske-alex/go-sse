# MongoDB Authentication for go-sse

This document explains how to configure MongoDB authentication with go-sse to ensure secure access to your data.

## Authentication Options

The go-sse server supports multiple ways to configure MongoDB authentication:

### Option 1: Full Connection String

The most flexible way is to provide a complete MongoDB connection string with credentials:

```
MONGO_URI=mongodb://username:password@hostname:port/database?authSource=admin
```

Example:
```
MONGO_URI=mongodb://myuser:mypassword@mongodb.example.com:27017/gosse?authSource=admin&replicaSet=rs0
```

This approach allows you to specify all MongoDB connection parameters in a single string, including:
- Authentication credentials
- Replica set configuration
- TLS/SSL options
- Connection pool settings
- Read preferences

### Option 2: Individual Components

Alternatively, you can specify the connection details as separate environment variables:

```
MONGO_HOST=hostname
MONGO_PORT=port
MONGO_USER=username
MONGO_PASSWORD=password
MONGO_AUTH_DB=admin  # Database used for authentication (typically 'admin')
```

Example:
```
MONGO_HOST=mongodb.example.com
MONGO_PORT=27017
MONGO_USER=myuser
MONGO_PASSWORD=mypassword
MONGO_AUTH_DB=admin
```

The application will automatically build a connection string from these components.

## Authentication with Docker Compose

The included docker-compose.yml configures MongoDB with basic authentication:

```yaml
# MongoDB service configuration
mongo:
  image: mongo:5
  environment:
    - MONGO_INITDB_DATABASE=gosse
    - MONGO_INITDB_ROOT_USERNAME=admin
    - MONGO_INITDB_ROOT_PASSWORD=password
```

You can set custom credentials by providing these environment variables when starting the containers:

```bash
MONGO_USER=myuser MONGO_PASSWORD=mypassword docker-compose --profile with-mongo up -d
```

## Authentication with MongoDB Atlas

To connect to MongoDB Atlas:

1. Create a MongoDB Atlas account and cluster
2. Create a database user in the Atlas UI
3. Get the connection string from Atlas
4. Set the `MONGO_URI` environment variable to the Atlas connection string

Example Atlas connection string:
```
MONGO_URI=mongodb+srv://username:password@cluster0.mongodb.net/gosse?retryWrites=true&w=majority
```

## Security Best Practices

1. **Use strong passwords** - Create secure, random passwords for MongoDB users

2. **Least privilege** - Create a dedicated database user for the application with only the necessary permissions

3. **Environment variables** - Store credentials in environment variables instead of hardcoding them

4. **Network security** - Use firewalls and VPCs to restrict access to your MongoDB instances

5. **TLS/SSL** - Enable TLS/SSL encryption for MongoDB connections by including these options in your connection string:
   ```
   MONGO_URI=mongodb://username:password@hostname:port/database?ssl=true&tlsCAFile=/path/to/ca.pem
   ```

6. **Audit logging** - Enable MongoDB audit logging to track access and changes

## Troubleshooting Authentication Issues

If you encounter connection problems:

1. **Check credentials** - Verify username, password, and authentication database

2. **Connection string** - Make sure the URI format is correct

3. **Network access** - Ensure the MongoDB server is accessible from the application

4. **Logs** - Check the go-sse server logs for detailed error messages

5. **MongoDB logs** - Examine MongoDB server logs for authentication failures

Common error messages and solutions:

- **Authentication failed** - Check username and password
- **Not authorized** - User may not have access to the specified database or collection
- **Connection refused** - MongoDB server may not be running or is blocked by a firewall

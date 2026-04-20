# Quick-Note API (Go + RavenDB)

![Go](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)
![RavenDB](https://img.shields.io/badge/RavenDB-Latest-37B7C3?style=flat&logo=database)
![Docker](https://img.shields.io/badge/Docker-Multi--Stage-2496ED?style=flat&logo=docker)
![License](https://img.shields.io/badge/License-MIT-blue?style=flat)

## Project Overview

This repository implements **Stack 2** of the **Quick-Note Polyglot** project ecosystem. It serves as a strictly decoupled REST API backend designed to operate independently from the React frontend application (typically running on localhost:5173). This architecture represents a fundamental alternative to Stack 1 (Spring Boot + MySQL), demonstrating how the same domain problem can be solved using a NoSQL paradigm combined with the Go language ecosystem.

The Quick-Note API provides a minimal but production-grade authentication and note management service, exposing seven REST endpoints that enforce strict contracts between frontend and backend. All state is persisted to RavenDB, a document-oriented NoSQL database that offers sophisticated querying capabilities while introducing important paradigm shifts in application design, particularly around eventual consistency and document sessions.

## System Architecture & Design Patterns

### Architectural Overview

The Quick-Note API is constructed using the following architectural principles:

- **Vertical Slice Architecture**: Each feature (authentication, note management) is organized as a vertical slice containing handlers, request/response models, and database operations.
- **Handler-Driven Routing**: The Gin web framework routes HTTP requests directly to handler functions, which receive a *gin.Context parameter for request/response manipulation.
- **Document Session Pattern**: RavenDB operations follow a session-based model where queries and mutations occur within the context of a document session, ensuring transactional consistency at the document level.
- **Seam Functions for Testing**: Database operations are abstracted behind seam functions (e.g., queryUsersByUsername, saveNote), enabling unit tests to mock RavenDB without spinning up a live database instance.

### Paradigm Shift: SQL vs NoSQL

#### Relational Model (Stack 1: Spring/MySQL)
Stack 1 employs normalized relational schemas with referential integrity constraints:
- Tables: Users, Notes, foreign key relationships enforced at the database layer
- Schema evolution requires migration scripts
- Transactions provide immediate consistency guarantees (ACID)
- Query results are immediately consistent after commit

#### Document Model (Stack 2: Go/RavenDB)
Stack 2 leverages a document-oriented NoSQL paradigm:
- Collections: Users and Notes stored as JSON documents without enforced relationships
- Schema evolution is implicit (documents can have different fields)
- Transactions operate at the document level; distributed consistency is eventual
- Query indexes are built asynchronously, introducing a window of staleness

### Handling Eventual Consistency: Non-Stale Query Results

RavenDB, like most NoSQL systems, introduces the concept of eventual consistency. When a document is stored via a session and SaveChanges is invoked, the write is durable. However, the indexes that support queries are updated asynchronously, creating a race condition:

**Scenario**: User registers, then immediately logs in.
1. Register endpoint stores the new User document
2. Login endpoint queries for the user by username via an index
3. The index has not yet been updated with the new user document
4. Login query returns no result, despite the user being registered

**Solution**: WaitForNonStaleResults(0)

All RavenDB queries in this codebase explicitly call `.WaitForNonStaleResults(0)` before retrieving results:

```go
var user User
err := session.Query[User]().
    WhereEquals("username", username).
    WaitForNonStaleResults(0).  // Block until index is updated
    First(&user)
```

This directive forces the query to wait synchronously until the index is refreshed, eliminating the race condition and providing read-after-write consistency at the application level. This demonstrates that even NoSQL systems can enforce consistency semantics through careful handling of eventual-consistency windows.

### Docker Multi-Stage Build Optimization

The Dockerfile employs a multi-stage build strategy to minimize runtime image size:

**Stage 1 (Builder)**:
- Base image: golang:1.22-alpine (includes full Go toolchain, ~400MB)
- Operations: Dependency download, compilation with CGO_ENABLED=0
- Output: Static binary (no runtime dependencies)

**Stage 2 (Runtime)**:
- Base image: alpine:latest (~5MB)
- Operations: Copy binary from Stage 1
- Result: Final image ~10-20MB (vs 400+MB if using golang image directly)

This approach reduces deployment footprint, startup time, and attack surface by excluding the entire Go toolchain and standard library from the runtime environment.

### Struct Serialization and JSON Mapping

Go's strict typing and explicit field export rules require careful attention to JSON mapping:

```go
type User struct {
    ID       string `json:"id"`
    Username string `json:"username"`
    Password string `json:"password"`
}
```

- Fields must be capitalized (exported) to be accessible outside the package
- JSON tags define the lowercase convention expected by REST clients
- RavenDB WhereEquals queries must use the JSON field name ("username"), not the Go struct field name ("Username")
- Bidirectional mapping ensures request JSON unmarshals correctly and response JSON marshals with expected casing

## API Contract

The API exposes seven endpoints with strict HTTP semantics. All requests and responses use JSON serialization. The base path for all endpoints is `/api`.

| Method | Endpoint | Request Body | Response (Success) | Status | Description |
|--------|----------|--------------|-------------------|--------|-------------|
| POST | /auth/login | {username, password} | {token, userId, username} | 200 | Authenticate user and return session token |
| POST | /auth/register | {username, password} | {message: "Registration successful. Please log in."} | 201 | Register new user; no token returned |
| GET | /notes | (none) | [{id, title, content, userId, isPinned, createdAt}, ...] | 200 | Retrieve all notes for authenticated user |
| POST | /notes | {title, content} | {id, title, content, userId, isPinned, createdAt} | 201 | Create new note with auto-generated UUID and timestamp |
| PUT | /notes/:id | {title, content} | {id, title, content, userId, isPinned, createdAt} | 200 | Update existing note; preserves isPinned and createdAt |
| PUT | /notes/:id/pin | {isPinned} | {id, title, content, userId, isPinned, createdAt} | 200 | Toggle pin status of existing note |
| DELETE | /notes/:id | (none) | {message: "Note deleted successfully", id} | 200 | Delete note by ID |

### Error Responses

All error responses follow a consistent structure:

- **400 Bad Request**: Invalid JSON, missing required fields, malformed request
- **401 Unauthorized**: Incorrect password during login
- **404 Not Found**: User not registered (login), note not found (get/update/delete)
- **500 Internal Server Error**: Database operation failure, persistence error

Error responses include a "message" field with descriptive text. Example:

```json
{
  "message": "User is not registered. Please register first."
}
```

## Prerequisites

### System Requirements

The following software must be installed on the host machine:

1. **Go Toolkit**: Go 1.22 or later
   - Download: https://golang.org/dl/
   - Verify installation: `go version`

2. **Docker**: Latest stable version
   - Download: https://www.docker.com/products/docker-desktop
   - Verify installation: `docker --version`

3. **Git**: For repository cloning
   - Verify installation: `git --version`

### Optional Tools

- **RavenDB Studio UI**: Accessible via web browser at http://localhost:8080 (when RavenDB container is running)
- **Postman** or **Thunder Client**: For manual API testing during development

## Local Execution

### Step 1: Start RavenDB Container

Launch the RavenDB container with security disabled (development only):

```bash
docker run \
  -e RAVEN_SECURITY_OMIT_SETUP=true \
  -p 8080:8080 \
  -p 38888:38888 \
  -d \
  ravendb/ravendb:latest
```

This command:
- Exposes the RavenDB management UI on port 8080
- Exposes the RavenDB server on port 38888
- Runs in detached mode (-d), allowing continued terminal use

Verify the container is running:

```bash
docker ps | grep ravendb
```

### Step 2: Create Database

Navigate to the RavenDB Studio UI in your browser:

```
http://localhost:8080
```

In the left sidebar, click "Databases" and then the "New Database" button. Create a database with the following settings:

- **Database Name**: `quicknote_db`
- **Replication Factor**: 1 (default)

Click "Create". The database is now available for application connections.

### Step 3: Clone Repository and Install Dependencies

```bash
git clone <repository-url> quick-note-api-go-ravendb
cd quick-note-api-go-ravendb
go mod download
```

Verify all dependencies are present:

```bash
go mod verify
```

### Step 4: Start the Application

From the repository root, run:

```bash
go run main.go
```

Expected output:

```
[GIN-debug] Loaded HTML Templates (0): 
[GIN-debug] Listening and serving HTTP on :5000
```

The API server is now listening on http://localhost:5000. The server enforces CORS for requests from http://localhost:5173 only.

### Step 5: Verify API Connectivity

Test the login endpoint with curl (example request):

```bash
curl -X POST http://localhost:5000/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"testuser","password":"testpass"}'
```

Expected response (404, user not yet registered):

```json
{
  "message": "User is not registered. Please register first."
}
```

## Testing & Code Coverage

### Running Tests

Execute the full test suite:

```bash
go test ./... -v
```

The `-v` flag enables verbose output, showing each test case result.

### Running Tests with Coverage

Generate a coverage profile:

```bash
go test ./... -v -coverprofile=coverage.out
```

View the coverage report in the terminal:

```bash
go tool cover -func=coverage.out
```

Generate an interactive HTML coverage report:

```bash
go tool cover -html=coverage.out -o coverage.html
```

Open coverage.html in your browser to visualize covered and uncovered code paths.

### Test Structure

The codebase includes comprehensive table-driven unit tests:

- **handlers/auth_handler_test.go**: Tests for Login and Register endpoints
- **handlers/note_handler_test.go**: Tests for all note CRUD operations

All tests use Go's standard httptest package to simulate HTTP requests without spinning up a live server. Database operations are mocked using seam functions, allowing tests to run in isolation.

### Test Coverage Goals

- **handlers/auth_handler.go**: 100% line coverage
- **handlers/note_handler.go**: 100% line coverage
- **models/*.go**: Implicit coverage through handler tests
- **db/database.go**: Covered indirectly through integration

## Containerization

### Building the Docker Image

From the repository root:

```bash
docker build -t quick-note-api:latest .
```

Verify the image was created:

```bash
docker images | grep quick-note-api
```

Expected output shows an image size of approximately 10-20MB (multi-stage optimization).

### Running the Application in Docker

Start the application container, linking it to the RavenDB container:

```bash
docker run \
  --network host \
  -e RAVEN_DB_URL=http://localhost:8080 \
  -e RAVEN_DB_NAME=quicknote_db \
  -p 5000:5000 \
  quick-note-api:latest
```

Alternatively, if containers are on a custom network:

```bash
docker network create quicknote-net
docker run --network quicknote-net --name ravendb -d ravendb/ravendb:latest
docker run --network quicknote-net -p 5000:5000 quick-note-api:latest
```

The API server is now accessible on http://localhost:5000 from the host machine.

### Minimal Image Footprint

The multi-stage Dockerfile ensures the runtime image contains only the compiled binary and minimal system libraries:

- No Go toolchain
- No source code
- No development dependencies
- Final size: ~10-20MB vs 400+MB for golang base image

This minimization reduces deployment time, storage requirements, and attack surface.

## Project Structure

```
quick-note-api-go-ravendb/
├── main.go                          # Application entry point, router setup
├── dockerfile                       # Multi-stage build configuration
├── go.mod                          # Go module definition
├── go.sum                          # Dependency lock file
├── db/
│   └── database.go                 # RavenDB initialization and connection pooling
├── handlers/
│   ├── auth_handler.go             # Login and Register endpoints
│   ├── auth_handler_test.go        # Authentication tests
│   ├── note_handler.go             # CRUD endpoints for notes
│   └── note_handler_test.go        # Note operation tests
└── models/
    ├── user.go                     # User struct and auth request/response
    └── note.go                     # Note struct and note request/response
```

## Development Workflow

### Making Code Changes

1. Edit source files in the handlers/ or models/ directories
2. Run tests to verify changes: `go test ./... -v`
3. Start the application: `go run main.go`
4. Test endpoints locally with curl or Postman
5. Commit changes with descriptive messages

### Adding New Endpoints

1. Create handler function in appropriate file under handlers/
2. Add request/response structs to models/
3. Wire route in main.go: `router.POST("/api/path", handlers.HandlerFunc)`
4. Write table-driven tests in corresponding _test.go file
5. Run full test suite to ensure no regressions

### Database Schema Evolution

RavenDB does not enforce schema validation, allowing documents to evolve incrementally. When adding new fields to Note or User structs:

1. Add field to struct definition in models/
2. Update JSON tags with appropriate casing
3. Update handler logic to populate new field
4. Write tests for new functionality
5. Existing documents without the new field will deserialize with zero values (safe)

## Deployment Considerations

### Security Notes

**Warning**: This implementation stores passwords in plaintext for development purposes. **Production deployments must implement bcrypt hashing** or equivalent cryptographic password protection.

**Warning**: Authentication tokens are currently generated as UUID strings for development. **Production deployments must implement JWT (JSON Web Tokens)** with expiration, signing, and claims validation.

**CORS Configuration**: The API restricts cross-origin requests to http://localhost:5173 by default. Modify the CORS configuration in main.go for production deployments.

## Contributing

This project is part of the Quick-Note Polyglot ecosystem. Changes should:

- Maintain backward compatibility with the REST API contract
- Include comprehensive unit tests with table-driven patterns
- Follow Go conventions and idioms (gofmt, effective Go)
- Update tests when endpoint contracts change

## License

This project is provided under the MIT License.

## References

- **Gin Web Framework**: https://github.com/gin-gonic/gin
- **RavenDB Go Client**: https://github.com/ravendb/ravendb-go-client
- **Go Modules**: https://golang.org/doc/go1.11
- **RavenDB Documentation**: https://ravendb.net/docs/start/about
- **Docker Multi-Stage Builds**: https://docs.docker.com/build/building/multi-stage/

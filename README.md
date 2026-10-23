# CAgent

CAgent is a multi-user AI agent backend.

The project provides backend APIs. The integrating application is responsible for user login and JWT issuance.

## Layout

### Source

```text
.
├── cmd/main.go                 # Entry point, command-line flags, and version information
├── app/                        # Service assembly, HTTP startup, and shutdown
├── config/                     # Environment variables, defaults, and logging setup
├── api/                        # Gin routing and shared responses
│   ├── common/                 # JWT cookie authentication and request logging middleware
│   └── v1/                     # v1 routes, handlers, and request/response schemas
├── internal/
│   ├── service/                # Models, sessions, keys, and agent execution
│   │   ├── adapter/            # MongoDB ADK SessionService adapter
│   │   ├── prompts/            # Built-in prompts
│   │   └── ringbuffer/         # SSE producers, consumers, and buffer management
│   ├── store/                  # MongoDB data access
│   │   ├── request/            # Storage-layer request structures
│   │   └── schema/             # Collection names, BSON documents, and enums
│   └── tool/                   # Tool factory and local, MCP, A2A, and Skill adapters
├── pkg/                        # Shared filex, kms, llm, and skillx components
├── docs/                       # Swagger files and MongoDB session storage documentation
├── skills/                     # Optional local Skill content
├── go.mod / go.sum             # Go version and dependencies
└── LICENSE                     # Apache-2.0 license
```

### Runtime

All relative paths are resolved against the **process working directory at startup**. Start the service from the project
root during development and set a fixed working directory when deploying it.

```text
<working-directory>/
├── .env                        # Configuration; newly generated versioned keys are appended here
├── cagent[.exe]                # Executable built with the commands below
├── logs/
│   └── cagent.log              # Default log file; rotated backups are compressed
└── skills/                     # Automatically created if possible; may be empty
    └── <skill-name>/
        ├── SKILL.md            # YAML frontmatter containing name and description
        ├── references/         # Optional reference material
        ├── assets/             # Optional assets
        └── scripts/            # Optional scripts
```

## Environment

### Configuration Reference

| Variable                     | Required                    | Default           | Description                                                                                                                         |
|------------------------------|-----------------------------|-------------------|-------------------------------------------------------------------------------------------------------------------------------------|
| `CAGENT_SERVER_ID`           | Yes                         | None              | Service identifier; currently loaded into configuration only and does not route sessions between instances                          |
| `CAGENT_SERVER_HOST`         | No                          | `0.0.0.0`         | HTTP listen address                                                                                                                 |
| `CAGENT_SERVER_PORT`         | No                          | `8080`            | HTTP listen port, as an integer                                                                                                     |
| `CAGENT_SERVER_HEADER`       | Yes                         | None              | **Cookie name** containing the JWT; the implementation reads a cookie rather than an HTTP header                                    |
| `CAGENT_SERVER_SALT`         | Yes                         | None              | JWT verification secret; must match the issuer's signing secret                                                                     |
| `CAGENT_LOG_PATH`            | No                          | `logs/cagent.log` | Absolute log file path or path relative to the working directory                                                                    |
| `CAGENT_LOG_LEVEL`           | No                          | `info`            | Logrus level, such as `debug`, `info`, `warn`, or `error`                                                                           |
| `CAGENT_LOG_SIZE`            | No                          | `50`              | Maximum size of each log file, in MB                                                                                                |
| `CAGENT_LOG_BACKUP`          | No                          | `3`               | Maximum number of rotated log backups                                                                                               |
| `CAGENT_STORAGE_DB`          | Yes                         | None              | MongoDB database name, not the database type                                                                                        |
| `CAGENT_STORAGE_DSN`         | Yes                         | None              | MongoDB connection URI, including hosts, authentication, and replica set options as needed                                          |
| `CAGENT_STORAGE_MAXIDLECONN` | No                          | `200`             | Maps to MongoDB `MaxPoolSize`, the maximum number of connections in the pool                                                        |
| `CAGENT_STORAGE_MINIDLECONN` | No                          | `5`               | Maps to MongoDB `MinPoolSize`, the minimum number of connections in the pool                                                        |
| `CAGENT_STORAGE_IDLETIME`    | No                          | `5m`              | Maximum connection idle time, using Go duration syntax such as `30s` or `5m`                                                        |
| `CAGENT_STORAGE_DEBUG`       | No                          | `false`           | Enables debug-level MongoDB driver command logging; the global log level must also allow it                                         |
| `CAGENT_SECRET_V<N>`         | Required for model API keys | None              | KMS encryption key; `N` is a positive integer version such as `1` or `2`. The value is a standard Base64-encoded random 32-byte key |

### Example `.env`

```dotenv
CAGENT_SERVER_ID=cagent-local
CAGENT_SERVER_HOST=0.0.0.0
CAGENT_SERVER_PORT=8080
CAGENT_SERVER_HEADER=cagent_token
CAGENT_SERVER_SALT=replace_with_random_jwt_signing_secret

CAGENT_LOG_PATH=logs/cagent.log
CAGENT_LOG_LEVEL=info
CAGENT_LOG_SIZE=50
CAGENT_LOG_BACKUP=3

CAGENT_STORAGE_DB=cagent
CAGENT_STORAGE_DSN="mongodb://127.0.0.1:27017/?replicaSet=rs0"
CAGENT_STORAGE_MAXIDLECONN=200
CAGENT_STORAGE_MINIDLECONN=5
CAGENT_STORAGE_IDLETIME=5m
CAGENT_STORAGE_DEBUG=false

CAGENT_SECRET_V1=replace_with_base64_encoded_32_byte_key
```

KMS uses the highest configured version to encrypt new API keys and the version stored in each record to decrypt
existing keys. During rotation, retain older versions until the corresponding API keys have been re-encrypted. `.env` is
excluded by `.gitignore`; store and back it up securely.

## StartUp

### Prerequisites

- Go 1.27 or later, as specified in `go.mod`.
- A MongoDB replica set or sharded cluster supporting multi-document transactions, initialized as described in section
  6.
- `mongosh` for database initialization, plus `mongod` if running MongoDB locally.
- An accessible model service compatible with OpenAI Chat Completions for conversations.

Run the following commands from the project root. First, download dependencies:

```shell
go mod download
```

### Build

```bash
go build -trimpath -ldflags "-w -s" -o ./release/cagent ./cmd ./release/cagent -v
```

### Start

After configuring `.env` and initializing the database, start the service from the project root:

```bash
./release/cagent
```

During development, run directly from source:

```shell
go run ./cmd
```

Open the [Swagger page](http://localhost:8080/cagent/swagger/index.html) to inspect the APIs. The service starts its
shutdown sequence when it receives `SIGINT` or `SIGTERM`. Press `Ctrl+C` to stop a local process.

## MongoDB

Execute the following JavaScript in the `mongosh` session opened above. The script skips existing collections. Index
creation can be repeated when the name and definition match an existing index; existing data must satisfy the unique
constraints.

The collection names below match the `cagent_*` names used by the application. If an older initialization script
created unprefixed collections, run this script to provision the indexes on the actual application collections.

```javascript
const cagentDb = db.getSiblingDB("cagent");
const collections = [
    "cagent_model", "cagent_tool", "cagent_session", "cagent_message",
    "cagent_adk_session", "cagent_adk_event", "cagent_adk_app_state", "cagent_adk_user_state"
];
const existingCollections = new Set(cagentDb.getCollectionNames());
for (const name of collections) {
    if (!existingCollections.has(name)) {
        cagentDb.createCollection(name);
    }
}

// Required for ADK correctness: unique session identities, event IDs, and shared state records.
cagentDb.cagent_adk_session.createIndex(
    {app_name: 1, user_id: 1, session_id: 1},
    {unique: true, name: "uq_adk_session_identity"}
);
cagentDb.cagent_adk_event.createIndex(
    {session: 1, event_id: 1},
    {unique: true, name: "uq_adk_session_event_id"}
);
cagentDb.cagent_adk_event.createIndex(
    {session: 1, timestamp: -1, sequence: -1},
    {name: "ix_adk_session_event_history"}
);
cagentDb.cagent_adk_app_state.createIndex(
    {app_name: 1},
    {unique: true, name: "uq_adk_app_state"}
);
cagentDb.cagent_adk_user_state.createIndex(
    {app_name: 1, user_id: 1},
    {unique: true, name: "uq_adk_user_state"}
);

// Supporting indexes for application queries.
cagentDb.cagent_session.createIndex(
    {user: 1, _id: -1}, {name: "ix_session_user_id"}
);
cagentDb.cagent_message.createIndex(
    {session: 1, _id: -1}, {name: "ix_message_session_id"}
);
cagentDb.cagent_model.createIndex(
    {type: 1, enable: 1}, {name: "ix_model_type_enable"}
);
cagentDb.cagent_tool.createIndex(
    {enable: 1}, {name: "ix_tool_enable"}
);

// Inspect collections and indexes.
printjson(cagentDb.getCollectionNames());
for (const name of collections) {
    print(name);
    printjson(cagentDb.getCollection(name).getIndexes());
}
```

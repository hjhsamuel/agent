# CAgent

Multi-user AI agent backend. The integrating application handles login and JWT issuance.

## Quick start

Requires Go 1.27+, a MongoDB replica set or sharded cluster with transaction support, and an OpenAI Chat Completions-compatible model service.

Create `.env` in the project root using the [environment configuration](#environment) below.

Initialize [MongoDB](#mongodb), then run from the project root (relative paths use the working directory):

```shell
go run ./cmd
```

Or build and run:

```shell
go build -trimpath -ldflags "-w -s" -o ./release/cagent ./cmd

./release/cagent
```

API documentation: [Swagger](http://localhost:8080/cagent/swagger/index.html).

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
CAGENT_SERVER_ID=1
CAGENT_SERVER_HOST=0.0.0.0
CAGENT_SERVER_PORT=8080
CAGENT_SERVER_HEADER={Cookie key}
CAGENT_SERVER_SALT={The secret to parse JWT}

CAGENT_LOG_PATH=/app/logs/cagent.log
CAGENT_LOG_LEVEL=info
CAGENT_LOG_SIZE=50
CAGENT_LOG_BACKUP=3

CAGENT_STORAGE_DB=cagent
CAGENT_STORAGE_DSN="mongodb://127.0.0.1:27017/?replicaSet=rs0"
CAGENT_STORAGE_MAXIDLECONN=200
CAGENT_STORAGE_MINIDLECONN=5
CAGENT_STORAGE_IDLETIME=5m
CAGENT_STORAGE_DEBUG=false

CAGENT_SECRET_V1={The secret encoded with base64}
```

KMS uses the highest configured version to encrypt new API keys and the version stored in each record to decrypt
existing keys. During rotation, retain older versions until the corresponding API keys have been re-encrypted. `.env` is
excluded by `.gitignore`; store and back it up securely.

## Docker

```shell
docker build -t cagent:{tag} -f Dockerfile .

docker run \
  -d \
  --name cagent \
  -p 8080:8080 \
  --env-file .env \
  -v cagent-logs:/app/logs \
  -v cagent-skills:/app/skills \
  cagent:{tag}
```

Use a MongoDB address reachable from the container. To persist application-generated keys, also bind-mount an existing writable `.env` at `/app/.env`. Mount local skills at `/app/skills` when needed. The image contains only the backend.

## Optional UI

Requires Node.js 20+. Start the backend, then run:

```shell
cd ui

npm start
```

Open [the UI](http://127.0.0.1:5001). Set `CAGENT_API_URL` to override the backend address (`http://127.0.0.1:8080`); configure the UI port in `ui/config.json`.

Enter the JWT cookie name and token in **连接与认证**. Configure a model, its API key, and a main Agent before chatting. See [ui/README.md](ui/README.md) for details.

## Local tools

A local tool is a trusted executable built for the server's OS and architecture. Package the following files in a ZIP archive (flat or inside one enclosing directory):

```text
echo_tool/
  tool.json
  echo_tool       # Windows: echo_tool.exe
```

`tool.json` contains four fields: `name`, `description`, `parametersJsonSchema` (input JSON Schema), and `responseJsonSchema` (output JSON Schema):

```json
{
  "name": "echo_tool",
  "description": "Return the supplied message",
  "parametersJsonSchema": {
    "type": "object",
    "properties": {
      "message": {
        "type": "string", 
        "description": "Message to echo"
      }
    },
    "required": ["message"],
    "additionalProperties": false
  },
  "responseJsonSchema": {
    "type": "object",
    "properties": {
      "message": {
        "type": "string"
      }
    },
    "required": ["message"]
  }
}
```

The tool name, executable name and registered tool name must match. Names start with a letter and contain only letters, digits, `_` or `-` (up to 64 characters).

The service invokes the executable through the system shell, with its version directory as the working directory. Read one JSON object from **stdin**, write one JSON object to **stdout**, send logs to **stderr**, and exit nonzero on failure. For example, input `{"message":"hello"}` produces output `{"message":"hello"}`. Validate input inside the executable; these schemas describe the interface and do not replace runtime validation.

Host the archive at a downloadable URL and register it with the authenticated `POST /cagent/v1/tool` API:

```json
{
  "name": "echo_tool",
  "kind": 0,
  "url": "https://example.com/echo_tool.zip",
  "agent": "<agent-id>"
}
```

Creation downloads asynchronously; wait for `state: 1` before use. Updating the URL installs a new version under `tools/<Name>/<Version>`; existing runners keep their original version even after update or deletion. Startup removes unused versions before restoring runners. Persist `tools` (Docker: `/app/tools`) and give each service instance its own directory.

## MongoDB

Run in `mongosh` with the target database selected:

```javascript
const collections = [
    "cagent_agent", "cagent_model", "cagent_tool", "cagent_session", "cagent_message",
    "cagent_adk_session", "cagent_adk_event", "cagent_adk_app_state", "cagent_adk_user_state"
];
const existing = new Set(db.getCollectionNames());
for (const name of collections) {
    if (!existing.has(name)) db.createCollection(name);
}

// Required unique indexes and event history index.
db.cagent_adk_session.createIndex({app_name: 1, user_id: 1, session_id: 1},
    {unique: true, name: "uq_adk_session_identity"});
db.cagent_adk_event.createIndex({session: 1, event_id: 1},
    {unique: true, name: "uq_adk_session_event_id"});
db.cagent_adk_event.createIndex({session: 1, timestamp: -1, sequence: -1},
    {name: "ix_adk_session_event_history"});
db.cagent_adk_app_state.createIndex({app_name: 1},
    {unique: true, name: "uq_adk_app_state"});
db.cagent_adk_user_state.createIndex({app_name: 1, user_id: 1},
    {unique: true, name: "uq_adk_user_state"});

// Application query indexes.
db.cagent_agent.createIndex({type: 1}, {name: "ix_agent_type"});
db.cagent_agent.createIndex({parent: 1}, {name: "ix_agent_parent"});
db.cagent_tool.createIndex({agent: 1}, {name: "ix_tool_agent"});
db.cagent_session.createIndex({user: 1, _id: -1}, {name: "ix_session_user_id"});
db.cagent_message.createIndex({session: 1, _id: -1}, {name: "ix_message_session_id"});
db.cagent_model.createIndex({type: 1, enable: 1}, {name: "ix_model_type_enable"});
db.cagent_tool.createIndex({enable: 1}, {name: "ix_tool_enable"});
```

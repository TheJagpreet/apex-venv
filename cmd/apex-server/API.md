# apex-server — REST API Documentation

`apex-server` exposes every apex-venv sandbox operation as an HTTP/JSON API, enabling frontend applications and external services to manage sandboxes programmatically.

## Base URL

```
http://localhost:8080
```

Override the port with the `PORT` environment variable:

```bash
PORT=9090 ./apex-server
```

## Authentication

No authentication is required by default. Deploy behind a reverse proxy (e.g. nginx, Caddy) to add auth headers or TLS.

## CORS

All responses include `Access-Control-Allow-Origin: *` so that browser-based frontends can call the API directly. Preflight `OPTIONS` requests are handled automatically.

---

## Endpoints Overview

| Method   | Path                                | Description                                   |
|----------|-------------------------------------|-----------------------------------------------|
| `GET`    | `/health`                           | Health check                                  |
| `POST`   | `/api/sandboxes`                    | Create a new sandbox                          |
| `GET`    | `/api/sandboxes`                    | List all sandboxes                            |
| `GET`    | `/api/sandboxes/{id}/status`        | Get sandbox status                            |
| `DELETE` | `/api/sandboxes/{id}`               | Destroy a sandbox                             |
| `POST`   | `/api/sandboxes/{id}/exec`          | Execute a command (buffered output)           |
| `POST`   | `/api/sandboxes/{id}/exec/stream`   | Execute a command (streaming/interleaved)     |
| `POST`   | `/api/sandboxes/{id}/copy-to`       | Copy a file from host into the sandbox        |
| `POST`   | `/api/sandboxes/{id}/copy-from`     | Copy a file from the sandbox to the host      |

---

## Common Response Format

### Success

Every successful response returns JSON with an HTTP 2xx status code.

### Error

```json
{
  "error": "description of what went wrong"
}
```

| HTTP Status | Meaning                                      |
|-------------|----------------------------------------------|
| `400`       | Bad request (missing/invalid parameters)     |
| `404`       | Sandbox not found                            |
| `500`       | Internal server error (Podman failure, etc.) |

---

## Endpoints

### `GET /health`

Health check to verify the server is running.

**Request**

```
GET /health
```

**Response `200 OK`**

```json
{
  "service": "apex-server",
  "status": "ok"
}
```

---

### `POST /api/sandboxes`

Create a new sandbox container. Every sandbox is automatically labelled `apex-venv=true` so it can be queried separately from other containers.

**Request**

```
POST /api/sandboxes
Content-Type: application/json
```

| Field       | Type       | Required | Description                                                   |
|-------------|------------|----------|---------------------------------------------------------------|
| `image`     | `string`   | **Yes**  | Container image (e.g. `apex-venv/ubuntu`)                     |
| `name`      | `string`   | No       | Human-readable name                                           |
| `workdir`   | `string`   | No       | Working directory inside the container                        |
| `env`       | `string[]` | No       | Environment variables as `KEY=VALUE` pairs                    |
| `mounts`    | `string[]` | No       | Bind mounts as `source:target[:ro]`                           |
| `memory`    | `string`   | No       | Memory limit (e.g. `512m`, `2g`)                              |
| `cpus`      | `number`   | No       | CPU limit (e.g. `1.5`)                                        |
| `repo_url`  | `string`   | No       | Git repository URL to clone into the working directory        |
| `timeout`   | `string`   | No       | Auto-destroy duration (e.g. `30m`, `2h`). Go duration format. |

**Example**

```bash
curl -X POST http://localhost:8080/api/sandboxes \
  -H "Content-Type: application/json" \
  -d '{
    "image": "apex-venv/ubuntu",
    "name": "my-sandbox",
    "memory": "512m",
    "cpus": 2,
    "timeout": "1h"
  }'
```

**Response `201 Created`**

```json
{
  "sandbox_id": "a1b2c3d4e5f6...",
  "status": "created",
  "timeout": "1h"
}
```

---

### `GET /api/sandboxes`

List all sandboxes managed by apex-venv (filtered by the `apex-venv=true` label).

**Request**

```
GET /api/sandboxes
```

**Response `200 OK`**

```json
{
  "sandboxes": [
    {
      "id": "a1b2c3d4e5f6...",
      "name": "my-sandbox",
      "image": "apex-venv/ubuntu",
      "status": "running"
    },
    {
      "id": "f6e5d4c3b2a1...",
      "name": "dev-env",
      "image": "apex-venv/python-3.12",
      "status": "stopped"
    }
  ]
}
```

The `status` field is one of: `running`, `stopped`, `unknown`.

---

### `GET /api/sandboxes/{id}/status`

Get the current status of a specific sandbox.

**Request**

```
GET /api/sandboxes/{id}/status
```

| Parameter | Type     | Location | Description          |
|-----------|----------|----------|----------------------|
| `id`      | `string` | path     | Sandbox container ID |

**Example**

```bash
curl http://localhost:8080/api/sandboxes/a1b2c3d4e5f6/status
```

**Response `200 OK`**

```json
{
  "sandbox_id": "a1b2c3d4e5f6...",
  "status": "running"
}
```

---

### `DELETE /api/sandboxes/{id}`

Stop and remove a sandbox. If the sandbox has a pending auto-cleanup timeout, it is cancelled.

**Request**

```
DELETE /api/sandboxes/{id}
```

| Parameter | Type     | Location | Description          |
|-----------|----------|----------|----------------------|
| `id`      | `string` | path     | Sandbox container ID |

**Example**

```bash
curl -X DELETE http://localhost:8080/api/sandboxes/a1b2c3d4e5f6
```

**Response `200 OK`**

```json
{
  "sandbox_id": "a1b2c3d4e5f6...",
  "status": "destroyed"
}
```

---

### `POST /api/sandboxes/{id}/exec`

Execute a command inside a sandbox and return the buffered output after completion.

**Request**

```
POST /api/sandboxes/{id}/exec
Content-Type: application/json
```

| Field     | Type       | Required | Description                        |
|-----------|------------|----------|------------------------------------|
| `command` | `string`   | **Yes**  | Shell command to execute           |
| `workdir` | `string`   | No       | Working directory for the command  |
| `env`     | `string[]` | No       | Extra environment variables        |

The `command` string is passed to `sh -c`, so pipes, redirects, and chained commands work as expected.

| Parameter | Type     | Location | Description          |
|-----------|----------|----------|----------------------|
| `id`      | `string` | path     | Sandbox container ID |

**Example**

```bash
curl -X POST http://localhost:8080/api/sandboxes/a1b2c3d4e5f6/exec \
  -H "Content-Type: application/json" \
  -d '{"command": "echo hello && uname -a"}'
```

**Response `200 OK`**

```json
{
  "exit_code": 0,
  "stdout": "hello\nLinux sandbox 6.1.0 ...\n",
  "stderr": ""
}
```

A non-zero `exit_code` is **not** treated as an error — the HTTP status is still `200`. Check `exit_code` in the response body.

---

### `POST /api/sandboxes/{id}/exec/stream`

Execute a command and return interleaved stdout/stderr output lines.

**Request**

```
POST /api/sandboxes/{id}/exec/stream
Content-Type: application/json
```

| Field     | Type       | Required | Description                        |
|-----------|------------|----------|------------------------------------|
| `command` | `string`   | **Yes**  | Shell command to execute           |
| `workdir` | `string`   | No       | Working directory for the command  |
| `env`     | `string[]` | No       | Extra environment variables        |

| Parameter | Type     | Location | Description          |
|-----------|----------|----------|----------------------|
| `id`      | `string` | path     | Sandbox container ID |

**Example**

```bash
curl -X POST http://localhost:8080/api/sandboxes/a1b2c3d4e5f6/exec/stream \
  -H "Content-Type: application/json" \
  -d '{"command": "echo out && echo err >&2"}'
```

**Response `200 OK`**

```json
{
  "exit_code": 0,
  "output": [
    {"stream": "stdout", "data": "out"},
    {"stream": "stderr", "data": "err"}
  ]
}
```

Each element in `output` represents one line of output with its source stream.

---

### `POST /api/sandboxes/{id}/copy-to`

Copy a file or directory from the host machine into a sandbox.

**Request**

```
POST /api/sandboxes/{id}/copy-to
Content-Type: application/json
```

| Field            | Type     | Required | Description                          |
|------------------|----------|----------|--------------------------------------|
| `host_path`      | `string` | **Yes**  | Absolute path on the host            |
| `container_path` | `string` | **Yes**  | Destination path inside the sandbox  |

| Parameter | Type     | Location | Description          |
|-----------|----------|----------|----------------------|
| `id`      | `string` | path     | Sandbox container ID |

**Example**

```bash
curl -X POST http://localhost:8080/api/sandboxes/a1b2c3d4e5f6/copy-to \
  -H "Content-Type: application/json" \
  -d '{"host_path": "/tmp/script.py", "container_path": "/workspace/script.py"}'
```

**Response `200 OK`**

```json
{
  "status": "copied",
  "host_path": "/tmp/script.py",
  "container_path": "/workspace/script.py"
}
```

---

### `POST /api/sandboxes/{id}/copy-from`

Copy a file or directory from a sandbox to the host machine.

**Request**

```
POST /api/sandboxes/{id}/copy-from
Content-Type: application/json
```

| Field            | Type     | Required | Description                          |
|------------------|----------|----------|--------------------------------------|
| `container_path` | `string` | **Yes**  | Path inside the sandbox              |
| `host_path`      | `string` | **Yes**  | Destination path on the host         |

| Parameter | Type     | Location | Description          |
|-----------|----------|----------|----------------------|
| `id`      | `string` | path     | Sandbox container ID |

**Example**

```bash
curl -X POST http://localhost:8080/api/sandboxes/a1b2c3d4e5f6/copy-from \
  -H "Content-Type: application/json" \
  -d '{"container_path": "/workspace/output.txt", "host_path": "/tmp/output.txt"}'
```

**Response `200 OK`**

```json
{
  "status": "copied",
  "host_path": "/tmp/output.txt",
  "container_path": "/workspace/output.txt"
}
```

---

## Label Filtering

Every sandbox created through the API (or the CLI / MCP server) is automatically labelled with `apex-venv=true`. The list endpoint returns **only** containers with this label, so apex-venv sandboxes are isolated from other Podman containers on the same host.

---

## Running the Server

### Build

```bash
go build -o apex-server ./cmd/apex-server/
```

### Run

```bash
./apex-server
# Listening on :8080

PORT=9090 ./apex-server
# Listening on :9090
```

### Install from source

```bash
go install github.com/apex-venv/apex-venv/cmd/apex-server@latest
```

---

## Example Frontend Workflow

A typical frontend dashboard would:

1. **Poll `GET /api/sandboxes`** to display a live table of all sandbox environments
2. **Call `POST /api/sandboxes`** when the user clicks "Create Sandbox"
3. **Call `GET /api/sandboxes/{id}/status`** to show real-time status for a selected sandbox
4. **Call `POST /api/sandboxes/{id}/exec`** to run commands and display output
5. **Call `DELETE /api/sandboxes/{id}`** when the user clicks "Destroy"

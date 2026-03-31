<div align="center">

# apex-venv

**Sandboxed container environments for AI agents to execute, test, and validate code — without touching the host.**

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Podman](https://img.shields.io/badge/Podman-4.0+-892CA0?logo=podman&logoColor=white)](https://podman.io)
[![MCP](https://img.shields.io/badge/MCP-Compatible-blue)](https://modelcontextprotocol.io)
[![License](https://img.shields.io/badge/License-TBD-lightgrey)](#license)

</div>

---

## Overview

apex-venv gives AI agents and developers secure, isolated container environments for running arbitrary code. Built on [Podman](https://podman.io), it provides three interfaces to the same core sandbox engine:

- **Go SDK** — Programmatic sandbox management with streaming output and automatic timeout cleanup
- **MCP Server** — Expose sandbox operations as tools for AI agents via the [Model Context Protocol](https://modelcontextprotocol.io)
- **REST API Server** — HTTP/JSON API for frontend applications and external services ([API docs](cmd/apex-server/API.md))
- **CLI** — Interactive terminal UI with real-time streaming output, color-coded status, and guided prompts
- **Pre-built Images** — Ubuntu, Python, and Node.js containers optimized for agent workloads

### Key Features

| Feature | Description |
|---------|-------------|
| **Container Isolation** | Each sandbox runs in its own Podman container with configurable resource limits |
| **Streaming Output** | Real-time, line-by-line command output via `ExecStream` — no waiting for completion |
| **Auto-Cleanup Timeouts** | Set a lifetime on sandboxes; they are automatically destroyed when the timeout expires |
| **Git Repo Cloning** | Clone a repository into the sandbox at creation time |
| **File Transfer** | Copy files and directories between the host and sandbox |
| **Resource Limits** | Constrain CPU and memory per sandbox |
| **MCP Integration** | Full suite of MCP tools for AI-agent-driven sandbox management |
| **REST API** | HTTP/JSON server for frontend dashboards and external integrations |
| **Interactive CLI** | Arrow-key menu navigation, prompts, spinners, and confirmation dialogs |

---

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
- [MCP Server](#mcp-server)
- [REST API Server](#rest-api-server)
- [Go SDK](#go-sdk)
- [Streaming Output](#streaming-output)
- [Sandbox Timeouts](#sandbox-timeouts)
- [Container Images](#container-images)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)

---

## Prerequisites

| Dependency | Version | Verify | Install |
|------------|---------|--------|---------|
| **Go** | 1.24+ | `go version` | [go.dev/doc/install](https://go.dev/doc/install) |
| **Podman** | 4.0+ | `podman info` | [podman.io/docs/installation](https://podman.io/docs/installation) |

---

## Installation

### Install from source

```bash
go install github.com/apex-venv/apex-venv/cmd/apex-venv@latest
```

This places the `apex-venv` binary in `$GOPATH/bin` (default `$HOME/go/bin`). Ensure that directory is in your `PATH`.

### Build locally

```bash
git clone https://github.com/apex-venv/apex-venv.git
cd apex-venv
go build -o apex-venv ./cmd/apex-venv/
```

On **Windows**, Go automatically produces `apex-venv.exe`:

```powershell
go build -o apex-venv.exe ./cmd/apex-venv/
```

### Cross-compile

```bash
# Linux (amd64)
GOOS=linux GOARCH=amd64 go build -o apex-venv-linux ./cmd/apex-venv/

# macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o apex-venv-darwin ./cmd/apex-venv/

# Windows
GOOS=windows GOARCH=amd64 go build -o apex-venv.exe ./cmd/apex-venv/
```

---

## Quick Start

**1. Build a sandbox image**

```bash
podman build -t apex-venv/ubuntu ./images/ubuntu/
```

Or build a Python / Node.js image:

```bash
podman build -t apex-venv/python-3.12 --build-arg PYTHON_VERSION=3.12 ./images/python/
podman build -t apex-venv/node-20     --build-arg NODE_VERSION=20     ./images/node/
```

**2. Create a sandbox**

```bash
apex-venv create --image apex-venv/ubuntu --name my-sandbox
```

**3. Create with a timeout (auto-cleanup)**

```bash
apex-venv create --image apex-venv/ubuntu --name temp --timeout 30m
```

The sandbox is automatically destroyed after 30 minutes.

**4. Run a command (streaming output)**

```bash
apex-venv exec <sandbox-id> -- echo "hello from sandbox"
```

Output streams to your terminal line-by-line in real time.

**5. Clone a repo into a sandbox**

```bash
apex-venv create --image apex-venv/python-3.12 --name dev \
  --repo https://github.com/user/project.git
```

**6. Clean up**

```bash
apex-venv destroy <sandbox-id>
```

Or run `apex-venv` with no arguments for an interactive guided experience.

---

## CLI Reference

### Usage

```
apex-venv                     Interactive mode (guided command picker)
apex-venv <command> [flags]   Direct mode
apex-venv help                Show help
```

### Commands

| Command | Description | Usage |
|---------|-------------|-------|
| `create` | Create a new sandbox | `apex-venv create --image <image> [flags]` |
| `list` | List all sandboxes | `apex-venv list` |
| `exec` | Run a command in a sandbox | `apex-venv exec <id> -- <command> [args...]` |
| `status` | Show sandbox status | `apex-venv status <id>` |
| `destroy` | Destroy a sandbox | `apex-venv destroy <id>` |
| `help` | Show usage information | `apex-venv help` |

### `create` Flags

| Flag | Description | Required |
|------|-------------|----------|
| `--image <image>` | Container image (e.g. `apex-venv/ubuntu`) | **Yes** |
| `--name <name>` | Human-readable sandbox name | No |
| `--workdir <dir>` | Working directory inside the container | No |
| `--env KEY=VAL` | Environment variable (repeatable) | No |
| `--mount src:dst[:ro]` | Bind mount from host to container (repeatable) | No |
| `--memory <limit>` | Memory limit (e.g. `512m`, `2g`) | No |
| `--cpus <n>` | CPU limit (e.g. `1.5`) | No |
| `--repo <url>` | Git repo URL to clone into the sandbox | No |
| `--timeout <duration>` | Auto-destroy after duration (e.g. `30m`, `2h`) | No |

If `--image` is omitted, the CLI prompts for it interactively.

### Examples

```bash
# Create with defaults
apex-venv create --image apex-venv/ubuntu --name dev

# Create with resource limits, mount, and timeout
apex-venv create --image apex-venv/ubuntu \
  --memory 512m --cpus 2 \
  --mount /home/user/project:/workspace \
  --env MY_VAR=hello \
  --timeout 1h

# Create a Python sandbox and clone a repo
apex-venv create --image apex-venv/python-3.12 \
  --repo https://github.com/user/project.git

# Create a Node.js sandbox with 30-minute timeout
apex-venv create --image apex-venv/node-20 --name node-dev --timeout 30m

# List all sandboxes
apex-venv list

# Execute a command (output streams in real time)
apex-venv exec abc123 -- ls -la /workspace
apex-venv exec abc123 -- bash -c "echo hello && uname -a"

# Check status
apex-venv status abc123

# Destroy
apex-venv destroy abc123
```

### Interactive Mode

Running `apex-venv` with no arguments launches interactive mode:

1. A command picker appears — navigate with arrow keys and press Enter
2. The CLI prompts for each required and optional field (including timeout)
3. Command output streams in real time
4. Destructive actions (like `destroy`) ask for confirmation

---

## MCP Server

`apex-mcp` is a [Model Context Protocol](https://modelcontextprotocol.io) server that lets AI agents create and manage sandbox environments over stdio.

### Build

```bash
go build -o apex-mcp ./cmd/apex-mcp/
```

Or install directly:

```bash
go install github.com/apex-venv/apex-venv/cmd/apex-mcp@latest
```

### Configuration

Add the server to your MCP client configuration. For Claude Desktop (`claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "apex-venv": {
      "command": "/path/to/apex-mcp"
    }
  }
}
```

Or run from source:

```json
{
  "mcpServers": {
    "apex-venv": {
      "command": "go",
      "args": ["run", "./cmd/apex-mcp/"],
      "cwd": "/path/to/apex-venv"
    }
  }
}
```

### Available Tools

| Tool | Description | Key Parameters |
|------|-------------|----------------|
| `create_sandbox` | Create a new sandbox container | `image` (required), `name`, `workdir`, `env`, `mounts`, `memory`, `cpus`, `repo_url`, `timeout` |
| `list_sandboxes` | List all apex-venv sandboxes | — |
| `exec_command` | Execute a command (buffered output) | `sandbox_id`, `command` (required), `workdir`, `env` |
| `exec_command_stream` | Execute a command (streaming output with interleaved stdout/stderr) | `sandbox_id`, `command` (required), `workdir`, `env` |
| `get_status` | Get sandbox status | `sandbox_id` (required) |
| `destroy_sandbox` | Stop and remove a sandbox | `sandbox_id` (required) |
| `copy_to_sandbox` | Copy file/directory host → sandbox | `sandbox_id`, `host_path`, `container_path` (all required) |
| `copy_from_sandbox` | Copy file/directory sandbox → host | `sandbox_id`, `container_path`, `host_path` (all required) |

### Example Agent Workflow

An AI agent connected via MCP can:

1. **Create a sandbox** — `create_sandbox` with `image: "apex-venv/python-3.12"`, optionally set `timeout: "1h"` and clone a repo
2. **Run commands** — `exec_command` for simple runs, or `exec_command_stream` for interleaved line-by-line output
3. **Transfer files** — `copy_to_sandbox` / `copy_from_sandbox` to move code and artifacts
4. **Check status** — `get_status` to verify the sandbox is running
5. **Clean up** — `destroy_sandbox` when done (or let the timeout handle it)

---

## REST API Server

`apex-server` is an HTTP/JSON server that exposes all apex-venv sandbox operations as REST endpoints — ideal for building frontend dashboards, CI/CD integrations, or any external service.

> **Full API reference:** [`cmd/apex-server/API.md`](cmd/apex-server/API.md)

### Build

```bash
go build -o apex-server ./cmd/apex-server/
```

Or install directly:

```bash
go install github.com/apex-venv/apex-venv/cmd/apex-server@latest
```

### Run

```bash
./apex-server          # listens on :8080
PORT=9090 ./apex-server  # override the port
```

### Endpoints

| Method   | Path                                | Description                               |
|----------|-------------------------------------|-------------------------------------------|
| `GET`    | `/health`                           | Health check                              |
| `POST`   | `/api/sandboxes`                    | Create a new sandbox                      |
| `GET`    | `/api/sandboxes`                    | List all sandboxes                        |
| `GET`    | `/api/sandboxes/{id}/status`        | Get sandbox status                        |
| `DELETE` | `/api/sandboxes/{id}`               | Destroy a sandbox                         |
| `POST`   | `/api/sandboxes/{id}/exec`          | Execute a command (buffered output)       |
| `POST`   | `/api/sandboxes/{id}/exec/stream`   | Execute a command (interleaved output)    |
| `POST`   | `/api/sandboxes/{id}/copy-to`       | Copy host → sandbox                       |
| `POST`   | `/api/sandboxes/{id}/copy-from`     | Copy sandbox → host                       |

All sandboxes are automatically labelled `apex-venv=true`, and the list endpoint returns only containers with that label.

### Quick Example

```bash
# Create a sandbox
curl -X POST http://localhost:8080/api/sandboxes \
  -H "Content-Type: application/json" \
  -d '{"image": "apex-venv/ubuntu", "name": "demo", "timeout": "30m"}'

# List sandboxes
curl http://localhost:8080/api/sandboxes

# Run a command
curl -X POST http://localhost:8080/api/sandboxes/<id>/exec \
  -H "Content-Type: application/json" \
  -d '{"command": "echo hello"}'

# Destroy
curl -X DELETE http://localhost:8080/api/sandboxes/<id>
```

---

## Go SDK

Import the `sandbox` package to manage sandboxes programmatically:

```bash
go get github.com/apex-venv/apex-venv/sandbox
```

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/apex-venv/apex-venv/sandbox"
)

func main() {
    ctx := context.Background()

    provider, err := sandbox.NewPodmanProvider()
    if err != nil {
        log.Fatal(err)
    }

    sb, err := provider.Create(ctx, sandbox.Config{
        Image:   "apex-venv/ubuntu",
        WorkDir: "/workspace",
    })
    if err != nil {
        log.Fatal(err)
    }
    defer sb.Destroy(ctx)

    result, err := sb.Exec(ctx, sandbox.Command{
        Cmd:  "bash",
        Args: []string{"-c", "echo hello from sandbox && uname -a"},
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("exit=%d\nstdout:\n%s\nstderr:\n%s\n",
        result.ExitCode, result.Stdout, result.Stderr)
}
```

### SDK Types

| Type | Description |
|------|-------------|
| `Provider` | Creates, lists, and retrieves sandboxes (`PodmanProvider` implementation) |
| `Sandbox` | A running container — exec commands, stream output, copy files, destroy |
| `Config` | Image, mounts, env vars, resource limits, name, workdir, repo URL, timeout |
| `Command` | Command to run: binary, args, dir, env, stdin |
| `ExecResult` | Exit code, stdout, stderr (from buffered `Exec`) |
| `OutputHandler` | Callback for streaming output: `func(stream string, data []byte)` |
| `Mount` | Host-to-container bind mount with optional read-only flag |
| `TimeoutManager` | Tracks sandbox lifetimes and triggers automatic cleanup |

---

## Streaming Output

The SDK provides two execution modes:

### Buffered Execution (`Exec`)

Runs the command and returns all output after completion:

```go
result, err := sb.Exec(ctx, sandbox.Command{
    Cmd:  "bash",
    Args: []string{"-c", "echo hello"},
})
// result.Stdout, result.Stderr, result.ExitCode
```

### Streaming Execution (`ExecStream`)

Delivers output line-by-line via a callback as it is produced:

```go
exitCode, err := sb.ExecStream(ctx, sandbox.Command{
    Cmd:  "bash",
    Args: []string{"-c", "for i in 1 2 3; do echo line $i; sleep 1; done"},
}, func(stream string, data []byte) {
    // stream is "stdout" or "stderr"
    fmt.Printf("[%s] %s\n", stream, string(data))
})
```

**When to use streaming:**
- Long-running commands where you want progress visibility
- Commands that produce large output you want to process incrementally
- Interactive CLI experiences where real-time feedback matters

The CLI uses streaming by default for all `exec` commands. The MCP server provides both `exec_command` (buffered) and `exec_command_stream` (streaming with interleaved output lines).

---

## Sandbox Timeouts

Sandboxes can be configured with a maximum lifetime. When the timeout expires, the sandbox is automatically destroyed — no manual cleanup required.

### CLI

```bash
# Auto-destroy after 30 minutes
apex-venv create --image apex-venv/ubuntu --timeout 30m

# Auto-destroy after 2 hours
apex-venv create --image apex-venv/python-3.12 --timeout 2h
```

### MCP

```json
{
  "tool": "create_sandbox",
  "arguments": {
    "image": "apex-venv/ubuntu",
    "timeout": "1h30m"
  }
}
```

### Go SDK

```go
import "time"

sb, err := provider.Create(ctx, sandbox.Config{
    Image:   "apex-venv/ubuntu",
    Timeout: 30 * time.Minute,
})
// Sandbox is automatically destroyed after 30 minutes.
// Manual sb.Destroy() cancels the pending timeout.
```

**How it works:**

1. When a sandbox is created with a timeout, the `TimeoutManager` schedules a cleanup timer
2. When the timer fires, the sandbox is automatically destroyed via `podman rm -f`
3. If the sandbox is manually destroyed before the timeout, the timer is cancelled
4. The timeout manager is thread-safe and supports concurrent sandbox creation

### Querying Remaining Time

```go
remaining, ok := provider.Timeouts().Remaining(sandboxID)
if ok {
    fmt.Printf("Sandbox expires in %s\n", remaining)
}
```

---

## Container Images

Pre-built Dockerfiles live in `images/`, optimized for agent workloads with common development tools pre-installed.

| Image | Base | Included Tools |
|-------|------|----------------|
| `apex-venv/ubuntu` | Ubuntu 24.04 | build-essential, git, curl, wget, jq, vim, python3, pip |
| `apex-venv/python-3.11` | Python 3.11 (slim) | build-essential, git, curl, wget, jq, vim, pip |
| `apex-venv/python-3.12` | Python 3.12 (slim) | build-essential, git, curl, wget, jq, vim, pip |
| `apex-venv/node-18` | Node.js 18 (slim) | build-essential, git, curl, wget, jq, vim, npm, python3 |
| `apex-venv/node-20` | Node.js 20 (slim) | build-essential, git, curl, wget, jq, vim, npm, python3 |
| `apex-venv/node-22` | Node.js 22 (slim) | build-essential, git, curl, wget, jq, vim, npm, python3 |

All images:
- Run as a non-root `sandbox` user with passwordless `sudo`
- Use `/workspace` as the default working directory
- Stay alive with `sleep infinity` for interactive `exec` usage

### Build Images

```bash
# Ubuntu
podman build -t apex-venv/ubuntu ./images/ubuntu/

# Python (specify version)
podman build -t apex-venv/python-3.11 --build-arg PYTHON_VERSION=3.11 ./images/python/
podman build -t apex-venv/python-3.12 --build-arg PYTHON_VERSION=3.12 ./images/python/

# Node.js (specify version)
podman build -t apex-venv/node-18 --build-arg NODE_VERSION=18 ./images/node/
podman build -t apex-venv/node-20 --build-arg NODE_VERSION=20 ./images/node/
podman build -t apex-venv/node-22 --build-arg NODE_VERSION=22 ./images/node/
```

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│  AI Agent  /  MCP Client  /  Developer Terminal      │
│  Frontend App  /  External Service                   │
└──────────────────────┬───────────────────────────────┘
                       │
           MCP (stdio) │  CLI commands  │  REST API (HTTP)
                       ▼
┌──────────────────────────────────────────────────────┐
│  apex-mcp (MCP Server)     apex-venv (CLI)           │
│  apex-server (REST API)                              │
│                                                      │
│  • 8 MCP tools             • Interactive mode        │
│  • 9 REST endpoints        • Streaming exec          │
│  • Streaming exec          • Timeout support         │
│  • Timeout support                                   │
└──────────────────────┬───────────────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────────────┐
│  sandbox (Go SDK)                                    │
│                                                      │
│  • Provider / Sandbox interfaces                     │
│  • Exec (buffered) & ExecStream (real-time)          │
│  • TimeoutManager (auto-cleanup)                     │
│  • File copy, resource limits, git clone             │
│  • Label: apex-venv=true on every sandbox            │
└──────────────────────┬───────────────────────────────┘
                       │  Podman CLI
                       ▼
┌──────────────────────────────────────────────────────┐
│  Podman Container Runtime                            │
│                                                      │
│  ubuntu  │  python-3.x  │  node-x                   │
│  (pre-built images from /images)                     │
└──────────────────────────────────────────────────────┘
```

---

## Project Structure

```
apex-venv/
├── cmd/
│   ├── apex-venv/              # CLI binary
│   │   └── main.go
│   ├── apex-mcp/               # MCP server binary
│   │   └── main.go
│   └── apex-server/            # REST API server binary
│       ├── main.go
│       └── API.md              # Full REST API documentation
├── sandbox/                    # Go SDK
│   ├── sandbox.go              # Interfaces & types (Sandbox, Provider, OutputHandler)
│   ├── config.go               # Config & Mount types (includes Timeout)
│   ├── podman.go               # Podman provider (Exec, ExecStream, lifecycle)
│   └── timeout.go              # TimeoutManager (auto-cleanup scheduler)
├── images/                     # Container image definitions
│   ├── ubuntu/Dockerfile
│   ├── python/Dockerfile       # Python 3.11 / 3.12 via build arg
│   └── node/Dockerfile         # Node.js 18 / 20 / 22 via build arg
├── go.mod
├── go.sum
└── README.md
```

---

## Roadmap

- [x] Core sandbox interface + Podman provider
- [x] Ubuntu, Python, and Node.js container images
- [x] CLI with interactive mode and color-coded output
- [x] Git repository cloning support
- [x] MCP tool definitions for AI agent integration
- [x] Streaming command output (`ExecStream`)
- [x] Sandbox timeout / auto-cleanup (`TimeoutManager`)
- [x] REST API server for frontend integrations (`apex-server`)
- [ ] Image registry (publish to ghcr.io)
- [ ] Snapshot / restore sandbox state
- [ ] Multi-provider support (Docker, containerd)

---

## Contributing

Contributions are welcome! To get started:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes and ensure they build (`go build ./...`)
4. Run `go vet ./...` to check for issues
5. Commit your changes (`git commit -m 'feat: add my feature'`)
6. Push to the branch (`git push origin feature/my-feature`)
7. Open a Pull Request

---

## License

TBD

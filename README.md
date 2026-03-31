# apex-venv

Sandboxed container environments for AI agents to execute, test, and validate code — without touching the host.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [CLI Reference](#cli-reference)
- [Go SDK](#go-sdk)
- [Container Images](#container-images)
- [Architecture](#architecture)
- [Project Structure](#project-structure)
- [Roadmap](#roadmap)

---

## Overview

apex-venv provides:

- **A Go SDK** (`sandbox/`) for creating and managing isolated Podman containers
- **A CLI** (`apex-venv`) with colorful interactive output for managing sandboxes from the terminal
- **Pre-built container images** (`images/`) optimized for agent workloads

---

## Prerequisites

| Dependency | Version | Check | Install |
|------------|---------|-------|---------|
| **Go** | 1.24+ | `go version` | [go.dev/doc/install](https://go.dev/doc/install) |
| **Podman** | 4.0+ | `podman info` | [podman.io/docs/installation](https://podman.io/docs/installation) |

---

## Installation

### Install from source

This installs the `apex-venv` binary to your `$GOPATH/bin` (or `$HOME/go/bin` by default). Make sure that directory is in your `PATH`.

```bash
go install github.com/apex-venv/apex-venv/cmd/apex-venv@latest
```

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
cd images/ubuntu
podman build -t apex-venv/ubuntu .
```

Or build a Python / Node.js image:

```bash
podman build -t apex-venv/python-3.12 --build-arg PYTHON_VERSION=3.12 ./images/python/
podman build -t apex-venv/node-20 --build-arg NODE_VERSION=20 ./images/node/
```

**2. Create a sandbox**

```bash
apex-venv create --image apex-venv/ubuntu --name my-sandbox
```

**3. Create a sandbox and clone a repo into it**

```bash
apex-venv create --image apex-venv/python-3.12 --name dev --repo https://github.com/user/project.git
```

**4. Run a command inside it**

```bash
apex-venv exec <sandbox-id> -- echo "hello from sandbox"
```

**5. Clean up**

```bash
apex-venv destroy <sandbox-id>
```

Or just run `apex-venv` with no arguments to get an interactive menu that walks you through everything.

---

## CLI Reference

### Usage

```
apex-venv                     Interactive mode (opens a command picker)
apex-venv <command> [flags]   Direct mode
apex-venv help                Show help
```

Running with no arguments opens an interactive picker where you can select a command with arrow keys, then fill in fields via prompts.

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
| `--image <image>` | Container image to use (e.g. `apex-venv/ubuntu`) | **Yes** |
| `--name <name>` | Human-readable sandbox name | No |
| `--workdir <dir>` | Working directory inside the container | No |
| `--env KEY=VAL` | Environment variable (repeatable) | No |
| `--mount src:dst[:ro]` | Bind mount from host to container (repeatable) | No |
| `--memory <limit>` | Memory limit (e.g. `512m`, `2g`) | No |
| `--cpus <n>` | CPU limit (e.g. `1.5`) | No |
| `--repo <url>` | Git repo URL to clone into the sandbox | No |

If `--image` is omitted, the CLI prompts for it interactively.

### Examples

```bash
# Create with defaults
apex-venv create --image apex-venv/ubuntu --name dev

# Create with resource limits and a mount
apex-venv create --image apex-venv/ubuntu \
  --memory 512m --cpus 2 \
  --mount /home/user/project:/workspace \
  --env MY_VAR=hello

# Create a Python sandbox and clone a repo
apex-venv create --image apex-venv/python-3.12 \
  --repo https://github.com/user/project.git

# Create a Node.js sandbox
apex-venv create --image apex-venv/node-20 --name node-dev

# List all sandboxes
apex-venv list

# Execute a command
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
2. The CLI prompts for each required and optional field
3. Destructive actions (like `destroy`) ask for confirmation before proceeding

---

## Go SDK

Import the `sandbox` package to manage sandboxes programmatically.

```bash
go get github.com/apex-venv/apex-venv/sandbox
```

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
        RepoURL: "https://github.com/user/project.git",
        Mounts: []sandbox.Mount{
            {Source: "/home/user/project", Target: "/workspace"},
        },
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
| `Provider` | Creates, lists, and retrieves sandboxes |
| `Sandbox` | A running container — exec commands, copy files, destroy |
| `Config` | Image, mounts, env vars, resource limits, name, workdir, repo URL |
| `Command` | Command to run: binary, args, dir, env, stdin |
| `ExecResult` | Exit code, stdout, stderr |
| `Mount` | Host-to-container bind mount with optional read-only flag |

---

## Container Images

Pre-built Dockerfiles optimized for agent workloads live in `images/`.

| Image | Base | What's Included |
|-------|------|-----------------|
| `ubuntu` | Ubuntu 24.04 | build-essential, git, curl, wget, jq, vim, python3, pip |
| `python-3.11` | Python 3.11 (Debian slim) | build-essential, git, curl, wget, jq, vim, pip |
| `python-3.12` | Python 3.12 (Debian slim) | build-essential, git, curl, wget, jq, vim, pip |
| `node-18` | Node.js 18 (Debian slim) | build-essential, git, curl, wget, jq, vim, npm, python3 |
| `node-20` | Node.js 20 (Debian slim) | build-essential, git, curl, wget, jq, vim, npm, python3 |
| `node-22` | Node.js 22 (Debian slim) | build-essential, git, curl, wget, jq, vim, npm, python3 |

Build an image:

```bash
# Ubuntu
podman build -t apex-venv/ubuntu ./images/ubuntu/

# Python (specify version with --build-arg)
podman build -t apex-venv/python-3.11 --build-arg PYTHON_VERSION=3.11 ./images/python/
podman build -t apex-venv/python-3.12 --build-arg PYTHON_VERSION=3.12 ./images/python/

# Node.js (specify version with --build-arg)
podman build -t apex-venv/node-18 --build-arg NODE_VERSION=18 ./images/node/
podman build -t apex-venv/node-20 --build-arg NODE_VERSION=20 ./images/node/
podman build -t apex-venv/node-22 --build-arg NODE_VERSION=22 ./images/node/
```

---

## Architecture

```
┌──────────────────────────────────────────────┐
│  Agent / MCP Tool / CLI                      │
│  (requests sandbox, sends commands)          │
└──────────────┬───────────────────────────────┘
               │
               ▼
┌──────────────────────────────────────────────┐
│  sandbox (Go SDK)                            │
│                                              │
│  • Create / destroy sandboxes                │
│  • Execute commands, capture output          │
│  • Mount host dirs, copy files in/out        │
│  • Resource limits (CPU, memory)             │
└──────────────┬───────────────────────────────┘
               │  Podman CLI
               ▼
┌──────────────────────────────────────────────┐
│  Container Runtime (Podman)                  │
│                                              │
│  Runs pre-built images from /images          │
└──────────────────────────────────────────────┘
```

---

## Project Structure

```
apex-venv/
├── cmd/
│   └── apex-venv/          # CLI binary
│       └── main.go
├── sandbox/                # Go SDK
│   ├── sandbox.go          # Sandbox interface & types
│   ├── config.go           # Configuration types
│   └── podman.go           # Podman provider implementation
├── images/                 # Pre-built container images
│   ├── ubuntu/
│   │   └── Dockerfile
│   ├── python/
│   │   └── Dockerfile      # Python 3.11 / 3.12 (via build arg)
│   └── node/
│       └── Dockerfile      # Node.js 18 / 20 / 22 (via build arg)
├── go.mod
└── README.md
```

---

## Roadmap

- [x] Core sandbox interface + Podman provider
- [x] Ubuntu base image
- [x] CLI with interactive mode
- [x] Python image (3.11, 3.12)
- [x] Node.js image (18, 20, 22)
- [x] Repo cloning support
- [ ] MCP tool definitions for agent integration
- [ ] Streaming command output
- [ ] Sandbox timeout / auto-cleanup
- [ ] Image registry (ghcr.io)

---

## License

TBD

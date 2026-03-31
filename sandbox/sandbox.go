package sandbox

import (
	"context"
	"io"
)

// OutputHandler is a callback invoked when streaming output is available.
// stream is either "stdout" or "stderr", and data is a line of output.
type OutputHandler func(stream string, data []byte)

// Sandbox represents an isolated container environment.
type Sandbox interface {
	// ID returns the unique container ID for this sandbox.
	ID() string

	// Exec runs a command inside the sandbox and returns the result.
	Exec(ctx context.Context, cmd Command) (*ExecResult, error)

	// ExecStream runs a command inside the sandbox and streams output
	// line-by-line to the provided handler. It returns the exit code.
	ExecStream(ctx context.Context, cmd Command, handler OutputHandler) (int, error)

	// CopyTo copies a file or directory from the host into the sandbox.
	CopyTo(ctx context.Context, hostPath, containerPath string) error

	// CopyFrom copies a file or directory from the sandbox to the host.
	CopyFrom(ctx context.Context, containerPath, hostPath string) error

	// Status returns the current status of the sandbox.
	Status(ctx context.Context) (SandboxStatus, error)

	// Destroy stops and removes the sandbox container.
	Destroy(ctx context.Context) error
}

// Provider manages the lifecycle of sandboxes.
type Provider interface {
	// Create creates and starts a new sandbox from the given config.
	Create(ctx context.Context, cfg Config) (Sandbox, error)

	// Get returns an existing sandbox by ID.
	Get(ctx context.Context, id string) (Sandbox, error)

	// List returns all sandboxes managed by this provider.
	List(ctx context.Context) ([]SandboxInfo, error)
}

// Command describes a command to execute inside a sandbox.
type Command struct {
	Cmd   string
	Args  []string
	Dir   string
	Env   []string
	Stdin io.Reader
}

// ExecResult holds the output of a command execution.
type ExecResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// SandboxStatus represents the state of a sandbox container.
type SandboxStatus string

const (
	StatusRunning SandboxStatus = "running"
	StatusStopped SandboxStatus = "stopped"
	StatusUnknown SandboxStatus = "unknown"
)

// SandboxInfo provides summary information about a sandbox.
type SandboxInfo struct {
	ID     string
	Name   string
	Image  string
	Status SandboxStatus
}

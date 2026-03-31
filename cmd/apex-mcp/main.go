package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/apex-venv/apex-venv/sandbox"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	provider, err := sandbox.NewPodmanProvider()
	if err != nil {
		log.Fatalf("failed to initialize sandbox provider: %v", err)
	}

	s := server.NewMCPServer(
		"apex-venv",
		"1.0.0",
		server.WithToolCapabilities(false),
	)

	registerTools(s, provider)

	if err := server.ServeStdio(s); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// registerTools registers all sandbox management tools with the MCP server.
func registerTools(s *server.MCPServer, provider sandbox.Provider) {
	// --- create_sandbox ---
	s.AddTool(
		mcp.NewTool("create_sandbox",
			mcp.WithDescription("Create a new sandbox container environment"),
			mcp.WithString("image",
				mcp.Required(),
				mcp.Description("Container image to use (e.g. apex-venv/ubuntu, apex-venv/python-3.12, apex-venv/node-20)"),
			),
			mcp.WithString("name",
				mcp.Description("Human-readable name for the sandbox"),
			),
			mcp.WithString("workdir",
				mcp.Description("Working directory inside the container (default: /workspace)"),
			),
			mcp.WithString("memory",
				mcp.Description("Memory limit (e.g. 512m, 2g)"),
			),
			mcp.WithNumber("cpus",
				mcp.Description("CPU limit (e.g. 1.5)"),
			),
			mcp.WithString("repo_url",
				mcp.Description("Git repository URL to clone into the working directory"),
			),
			mcp.WithArray("env",
				mcp.Description("Environment variables as KEY=VALUE strings"),
				mcp.WithStringItems(),
			),
			mcp.WithArray("mounts",
				mcp.Description("Bind mounts in the format source:target or source:target:ro"),
				mcp.WithStringItems(),
			),
			mcp.WithString("timeout",
				mcp.Description("Maximum sandbox lifetime (e.g. 30m, 2h, 1h30m). Sandbox is auto-destroyed after this duration. Empty means no timeout."),
			),
		),
		handleCreateSandbox(provider),
	)

	// --- list_sandboxes ---
	s.AddTool(
		mcp.NewTool("list_sandboxes",
			mcp.WithDescription("List all sandbox containers managed by apex-venv"),
		),
		handleListSandboxes(provider),
	)

	// --- exec_command ---
	s.AddTool(
		mcp.NewTool("exec_command",
			mcp.WithDescription("Execute a command inside a sandbox container"),
			mcp.WithString("sandbox_id",
				mcp.Required(),
				mcp.Description("The sandbox container ID or name"),
			),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("The command to execute (e.g. bash -c 'echo hello')"),
			),
			mcp.WithString("workdir",
				mcp.Description("Working directory for the command inside the container"),
			),
			mcp.WithArray("env",
				mcp.Description("Environment variables as KEY=VALUE strings for this command"),
				mcp.WithStringItems(),
			),
		),
		handleExecCommand(provider),
	)

	// --- get_status ---
	s.AddTool(
		mcp.NewTool("get_status",
			mcp.WithDescription("Get the current status of a sandbox container"),
			mcp.WithString("sandbox_id",
				mcp.Required(),
				mcp.Description("The sandbox container ID or name"),
			),
		),
		handleGetStatus(provider),
	)

	// --- destroy_sandbox ---
	s.AddTool(
		mcp.NewTool("destroy_sandbox",
			mcp.WithDescription("Stop and remove a sandbox container"),
			mcp.WithToolAnnotation(mcp.ToolAnnotation{
				DestructiveHint: boolPtr(true),
			}),
			mcp.WithString("sandbox_id",
				mcp.Required(),
				mcp.Description("The sandbox container ID or name"),
			),
		),
		handleDestroySandbox(provider),
	)

	// --- copy_to_sandbox ---
	s.AddTool(
		mcp.NewTool("copy_to_sandbox",
			mcp.WithDescription("Copy a file or directory from the host into a sandbox container"),
			mcp.WithString("sandbox_id",
				mcp.Required(),
				mcp.Description("The sandbox container ID or name"),
			),
			mcp.WithString("host_path",
				mcp.Required(),
				mcp.Description("Absolute path on the host to copy from"),
			),
			mcp.WithString("container_path",
				mcp.Required(),
				mcp.Description("Path inside the container to copy to"),
			),
		),
		handleCopyToSandbox(provider),
	)

	// --- copy_from_sandbox ---
	s.AddTool(
		mcp.NewTool("copy_from_sandbox",
			mcp.WithDescription("Copy a file or directory from a sandbox container to the host"),
			mcp.WithString("sandbox_id",
				mcp.Required(),
				mcp.Description("The sandbox container ID or name"),
			),
			mcp.WithString("container_path",
				mcp.Required(),
				mcp.Description("Path inside the container to copy from"),
			),
			mcp.WithString("host_path",
				mcp.Required(),
				mcp.Description("Absolute path on the host to copy to"),
			),
		),
		handleCopyFromSandbox(provider),
	)

	// --- exec_command_stream ---
	s.AddTool(
		mcp.NewTool("exec_command_stream",
			mcp.WithDescription("Execute a command inside a sandbox container with streaming output. Returns interleaved stdout/stderr lines as they are produced."),
			mcp.WithString("sandbox_id",
				mcp.Required(),
				mcp.Description("The sandbox container ID or name"),
			),
			mcp.WithString("command",
				mcp.Required(),
				mcp.Description("The command to execute (e.g. bash -c 'echo hello')"),
			),
			mcp.WithString("workdir",
				mcp.Description("Working directory for the command inside the container"),
			),
			mcp.WithArray("env",
				mcp.Description("Environment variables as KEY=VALUE strings for this command"),
				mcp.WithStringItems(),
			),
		),
		handleExecCommandStream(provider),
	)
}

// --- Tool Handlers ---

func handleCreateSandbox(provider sandbox.Provider) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		image, err := req.RequireString("image")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		cfg := sandbox.Config{
			Image:   image,
			Name:    req.GetString("name", ""),
			WorkDir: req.GetString("workdir", ""),
			Memory:  req.GetString("memory", ""),
			CPUs:    req.GetFloat("cpus", 0),
			RepoURL: req.GetString("repo_url", ""),
			Env:     req.GetStringSlice("env", nil),
		}

		if timeoutStr := req.GetString("timeout", ""); timeoutStr != "" {
			d, parseErr := time.ParseDuration(timeoutStr)
			if parseErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid timeout %q: %v", timeoutStr, parseErr)), nil
			}
			cfg.Timeout = d
		}

		mountStrs := req.GetStringSlice("mounts", nil)
		for _, ms := range mountStrs {
			m, parseErr := parseMount(ms)
			if parseErr != nil {
				return mcp.NewToolResultError(fmt.Sprintf("invalid mount %q: %v", ms, parseErr)), nil
			}
			cfg.Mounts = append(cfg.Mounts, m)
		}

		sb, err := provider.Create(ctx, cfg)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to create sandbox: %v", err)), nil
		}

		result := map[string]string{
			"sandbox_id": sb.ID(),
			"status":     "created",
		}
		if cfg.Timeout > 0 {
			result["timeout"] = cfg.Timeout.String()
		}

		return jsonResult(result)
	}
}

func handleListSandboxes(provider sandbox.Provider) server.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		infos, err := provider.List(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to list sandboxes: %v", err)), nil
		}

		type entry struct {
			ID     string `json:"id"`
			Name   string `json:"name,omitempty"`
			Image  string `json:"image"`
			Status string `json:"status"`
		}
		out := make([]entry, len(infos))
		for i, info := range infos {
			out[i] = entry{
				ID:     info.ID,
				Name:   info.Name,
				Image:  info.Image,
				Status: string(info.Status),
			}
		}
		return jsonResult(out)
	}
}

func handleExecCommand(provider sandbox.Provider) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("sandbox_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		rawCmd, err := req.RequireString("command")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sb, err := provider.Get(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sandbox not found: %v", err)), nil
		}

		cmd := sandbox.Command{
			Cmd:  "sh",
			Args: []string{"-c", rawCmd},
			Dir:  req.GetString("workdir", ""),
			Env:  req.GetStringSlice("env", nil),
		}

		result, err := sb.Exec(ctx, cmd)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("exec failed: %v", err)), nil
		}

		return jsonResult(map[string]any{
			"exit_code": result.ExitCode,
			"stdout":    result.Stdout,
			"stderr":    result.Stderr,
		})
	}
}

func handleGetStatus(provider sandbox.Provider) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("sandbox_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sb, err := provider.Get(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sandbox not found: %v", err)), nil
		}

		status, err := sb.Status(ctx)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to get status: %v", err)), nil
		}

		return jsonResult(map[string]string{
			"sandbox_id": id,
			"status":     string(status),
		})
	}
}

func handleDestroySandbox(provider sandbox.Provider) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("sandbox_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sb, err := provider.Get(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sandbox not found: %v", err)), nil
		}

		if err := sb.Destroy(ctx); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("failed to destroy sandbox: %v", err)), nil
		}

		return jsonResult(map[string]string{
			"sandbox_id": id,
			"status":     "destroyed",
		})
	}
}

func handleCopyToSandbox(provider sandbox.Provider) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("sandbox_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		hostPath, err := req.RequireString("host_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		containerPath, err := req.RequireString("container_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sb, err := provider.Get(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sandbox not found: %v", err)), nil
		}

		if err := sb.CopyTo(ctx, hostPath, containerPath); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("copy failed: %v", err)), nil
		}

		return jsonResult(map[string]string{
			"status":         "copied",
			"host_path":      hostPath,
			"container_path": containerPath,
		})
	}
}

func handleCopyFromSandbox(provider sandbox.Provider) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("sandbox_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		containerPath, err := req.RequireString("container_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		hostPath, err := req.RequireString("host_path")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sb, err := provider.Get(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sandbox not found: %v", err)), nil
		}

		if err := sb.CopyFrom(ctx, containerPath, hostPath); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("copy failed: %v", err)), nil
		}

		return jsonResult(map[string]string{
			"status":         "copied",
			"container_path": containerPath,
			"host_path":      hostPath,
		})
	}
}

func handleExecCommandStream(provider sandbox.Provider) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("sandbox_id")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		rawCmd, err := req.RequireString("command")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		sb, err := provider.Get(ctx, id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("sandbox not found: %v", err)), nil
		}

		cmd := sandbox.Command{
			Cmd:  "sh",
			Args: []string{"-c", rawCmd},
			Dir:  req.GetString("workdir", ""),
			Env:  req.GetStringSlice("env", nil),
		}

		type streamLine struct {
			Stream string `json:"stream"`
			Data   string `json:"data"`
		}

		var mu sync.Mutex
		var lines []streamLine

		handler := func(stream string, data []byte) {
			mu.Lock()
			lines = append(lines, streamLine{
				Stream: stream,
				Data:   string(data),
			})
			mu.Unlock()
		}

		exitCode, err := sb.ExecStream(ctx, cmd, handler)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("exec failed: %v", err)), nil
		}

		return jsonResult(map[string]any{
			"exit_code": exitCode,
			"output":    lines,
		})
	}
}

// --- Helpers ---

// parseMount parses a mount string in the format "source:target" or "source:target:ro".
func parseMount(s string) (sandbox.Mount, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return sandbox.Mount{}, fmt.Errorf("expected format source:target[:ro]")
	}
	m := sandbox.Mount{
		Source: parts[0],
		Target: parts[1],
	}
	if len(parts) == 3 && parts[2] == "ro" {
		m.ReadOnly = true
	}
	return m, nil
}

// jsonResult serializes v as indented JSON and returns it as a text tool result.
func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal result: %w", err)
	}
	return mcp.NewToolResultText(string(data)), nil
}

func boolPtr(b bool) *bool { return &b }

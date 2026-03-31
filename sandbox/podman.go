package sandbox

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// PodmanProvider implements Provider using Podman CLI.
type PodmanProvider struct {
	binary string // path to podman binary
}

// NewPodmanProvider creates a new PodmanProvider.
// It verifies that Podman is available on the system.
func NewPodmanProvider() (*PodmanProvider, error) {
	binary, err := exec.LookPath("podman")
	if err != nil {
		return nil, fmt.Errorf("podman not found in PATH: %w", err)
	}
	return &PodmanProvider{binary: binary}, nil
}

func (p *PodmanProvider) Create(ctx context.Context, cfg Config) (Sandbox, error) {
	args := []string{"run", "-d", "--label", "apex-venv=true"}

	if cfg.Name != "" {
		args = append(args, "--name", cfg.Name)
	}

	if cfg.WorkDir != "" {
		args = append(args, "--workdir", cfg.WorkDir)
	}

	if cfg.Memory != "" {
		args = append(args, "--memory", cfg.Memory)
	}

	if cfg.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", cfg.CPUs))
	}

	for _, env := range cfg.Env {
		args = append(args, "--env", env)
	}

	for _, m := range cfg.Mounts {
		mountStr := fmt.Sprintf("%s:%s", m.Source, m.Target)
		if m.ReadOnly {
			mountStr += ":ro"
		}
		args = append(args, "-v", mountStr)
	}

	// Keep container alive with a long sleep so we can exec into it.
	args = append(args, cfg.Image, "sleep", "infinity")

	stdout, stderr, err := p.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("podman run failed: %w\nstderr: %s", err, stderr)
	}

	id := strings.TrimSpace(stdout)
	sb := &podmanSandbox{
		id:       id,
		image:    cfg.Image,
		provider: p,
	}

	// Clone a git repository into the working directory if requested.
	if cfg.RepoURL != "" {
		cloneDir := cfg.WorkDir
		if cloneDir == "" {
			cloneDir = "/workspace"
		}
		cloneCmd := Command{
			Cmd:  "git",
			Args: []string{"clone", cfg.RepoURL, cloneDir},
		}
		result, err := sb.Exec(ctx, cloneCmd)
		if err != nil {
			// Clean up the container on clone failure.
			_ = sb.Destroy(ctx)
			return nil, fmt.Errorf("git clone failed: %w", err)
		}
		if result.ExitCode != 0 {
			_ = sb.Destroy(ctx)
			return nil, fmt.Errorf("git clone failed (exit %d): %s", result.ExitCode, result.Stderr)
		}
	}

	return sb, nil
}

func (p *PodmanProvider) Get(ctx context.Context, id string) (Sandbox, error) {
	// Verify the container exists by inspecting it.
	args := []string{"inspect", "--format", "{{.Config.Image}}", id}
	stdout, stderr, err := p.run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("sandbox %q not found: %w\nstderr: %s", id, err, stderr)
	}

	image := strings.TrimSpace(stdout)
	return &podmanSandbox{
		id:       id,
		image:    image,
		provider: p,
	}, nil
}

func (p *PodmanProvider) List(ctx context.Context) ([]SandboxInfo, error) {
	args := []string{"ps", "-a", "--format", "json", "--filter", "label=apex-venv=true"}
	stdout, stderr, err := p.run(ctx, args...)
	if err != nil {
		// If no containers match, podman may return empty output.
		if strings.TrimSpace(stdout) == "" || strings.TrimSpace(stdout) == "[]" {
			return nil, nil
		}
		return nil, fmt.Errorf("podman ps failed: %w\nstderr: %s", err, stderr)
	}

	if strings.TrimSpace(stdout) == "" || strings.TrimSpace(stdout) == "[]" {
		return nil, nil
	}

	var containers []struct {
		ID    string   `json:"Id"`
		Image string   `json:"Image"`
		State string   `json:"State"`
		Names []string `json:"Names"`
	}
	if err := json.Unmarshal([]byte(stdout), &containers); err != nil {
		return nil, fmt.Errorf("failed to parse podman output: %w", err)
	}

	infos := make([]SandboxInfo, len(containers))
	for i, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
		}
		infos[i] = SandboxInfo{
			ID:     c.ID,
			Name:   name,
			Image:  c.Image,
			Status: mapStatus(c.State),
		}
	}
	return infos, nil
}

func (p *PodmanProvider) run(ctx context.Context, args ...string) (string, string, error) {
	cmd := exec.CommandContext(ctx, p.binary, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// --- podmanSandbox ---

type podmanSandbox struct {
	id       string
	image    string
	provider *PodmanProvider
}

func (s *podmanSandbox) ID() string {
	return s.id
}

func (s *podmanSandbox) Exec(ctx context.Context, cmd Command) (*ExecResult, error) {
	args := []string{"exec"}

	if cmd.Dir != "" {
		args = append(args, "--workdir", cmd.Dir)
	}

	for _, env := range cmd.Env {
		args = append(args, "--env", env)
	}

	args = append(args, s.id, cmd.Cmd)
	args = append(args, cmd.Args...)

	podmanCmd := exec.CommandContext(ctx, s.provider.binary, args...)
	var stdout, stderr bytes.Buffer
	podmanCmd.Stdout = &stdout
	podmanCmd.Stderr = &stderr

	if cmd.Stdin != nil {
		podmanCmd.Stdin = cmd.Stdin
	}

	err := podmanCmd.Run()

	result := &ExecResult{
		Stdout: stdout.String(),
		Stderr: stderr.String(),
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitErr.ExitCode()
			return result, nil
		}
		return nil, fmt.Errorf("exec failed: %w", err)
	}

	result.ExitCode = 0
	return result, nil
}

func (s *podmanSandbox) CopyTo(ctx context.Context, hostPath, containerPath string) error {
	dst := fmt.Sprintf("%s:%s", s.id, containerPath)
	_, stderr, err := s.provider.run(ctx, "cp", hostPath, dst)
	if err != nil {
		return fmt.Errorf("copy to sandbox failed: %w\nstderr: %s", err, stderr)
	}
	return nil
}

func (s *podmanSandbox) CopyFrom(ctx context.Context, containerPath, hostPath string) error {
	src := fmt.Sprintf("%s:%s", s.id, containerPath)
	_, stderr, err := s.provider.run(ctx, "cp", src, hostPath)
	if err != nil {
		return fmt.Errorf("copy from sandbox failed: %w\nstderr: %s", err, stderr)
	}
	return nil
}

func (s *podmanSandbox) Status(ctx context.Context) (SandboxStatus, error) {
	stdout, stderr, err := s.provider.run(ctx, "inspect", "--format", "{{.State.Status}}", s.id)
	if err != nil {
		return StatusUnknown, fmt.Errorf("inspect failed: %w\nstderr: %s", err, stderr)
	}
	return mapStatus(strings.TrimSpace(stdout)), nil
}

func (s *podmanSandbox) Destroy(ctx context.Context) error {
	_, stderr, err := s.provider.run(ctx, "rm", "-f", s.id)
	if err != nil {
		return fmt.Errorf("destroy failed: %w\nstderr: %s", err, stderr)
	}
	return nil
}

func mapStatus(state string) SandboxStatus {
	switch strings.ToLower(state) {
	case "running":
		return StatusRunning
	case "exited", "stopped", "created":
		return StatusStopped
	default:
		return StatusUnknown
	}
}

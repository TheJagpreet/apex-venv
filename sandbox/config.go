package sandbox

// Config holds the configuration for creating a new sandbox.
type Config struct {
	// Image is the container image to use (e.g. "apex-venv/ubuntu").
	Image string

	// WorkDir is the working directory inside the container.
	WorkDir string

	// Mounts specifies host directories to mount into the container.
	Mounts []Mount

	// Env specifies environment variables as KEY=VALUE pairs.
	Env []string

	// Memory sets the memory limit (e.g. "512m", "2g"). Empty means no limit.
	Memory string

	// CPUs limits the number of CPUs. 0 means no limit.
	CPUs float64

	// Name is an optional human-readable name for the sandbox.
	Name string

	// RepoURL is an optional Git repository URL to clone into the working directory.
	RepoURL string
}

// Mount describes a bind mount from host to container.
type Mount struct {
	// Source is the absolute path on the host.
	Source string

	// Target is the path inside the container.
	Target string

	// ReadOnly makes the mount read-only inside the container.
	ReadOnly bool
}

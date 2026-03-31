package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/apex-venv/apex-venv/sandbox"
)

// --- Request / Response types ---

type createRequest struct {
	Image   string   `json:"image"`
	Name    string   `json:"name,omitempty"`
	WorkDir string   `json:"workdir,omitempty"`
	Env     []string `json:"env,omitempty"`
	Mounts  []string `json:"mounts,omitempty"`
	Memory  string   `json:"memory,omitempty"`
	CPUs    float64  `json:"cpus,omitempty"`
	RepoURL string   `json:"repo_url,omitempty"`
	Timeout string   `json:"timeout,omitempty"`
}

type createResponse struct {
	SandboxID string `json:"sandbox_id"`
	Status    string `json:"status"`
	Timeout   string `json:"timeout,omitempty"`
}

type sandboxEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	Status string `json:"status"`
}

type listResponse struct {
	Sandboxes []sandboxEntry `json:"sandboxes"`
}

type statusResponse struct {
	SandboxID string `json:"sandbox_id"`
	Status    string `json:"status"`
}

type execRequest struct {
	Command string   `json:"command"`
	WorkDir string   `json:"workdir,omitempty"`
	Env     []string `json:"env,omitempty"`
}

type execResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
}

type streamLine struct {
	Stream string `json:"stream"`
	Data   string `json:"data"`
}

type execStreamResponse struct {
	ExitCode int          `json:"exit_code"`
	Output   []streamLine `json:"output"`
}

type destroyResponse struct {
	SandboxID string `json:"sandbox_id"`
	Status    string `json:"status"`
}

type copyRequest struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
}

type copyResponse struct {
	Status        string `json:"status"`
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// --- Constants ---

const (
	maxRequestBodySize = 1 << 20 // 1 MB
	readHeaderTimeout  = 10 * time.Second
	shutdownTimeout    = 10 * time.Second
)

// --- Helpers ---

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

func readJSON(r *http.Request, v any) error {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxRequestBodySize))
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	if len(body) == 0 {
		return fmt.Errorf("request body is empty")
	}
	return json.Unmarshal(body, v)
}

func parseMount(s string) (sandbox.Mount, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return sandbox.Mount{}, fmt.Errorf("invalid mount format %q (expected source:target[:ro])", s)
	}
	m := sandbox.Mount{Source: parts[0], Target: parts[1]}
	if len(parts) == 3 && parts[2] == "ro" {
		m.ReadOnly = true
	}
	return m, nil
}

// --- Handlers ---

func handleCreateSandbox(provider *sandbox.PodmanProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Image == "" {
			writeError(w, http.StatusBadRequest, "image is required")
			return
		}

		cfg := sandbox.Config{
			Image:   req.Image,
			Name:    req.Name,
			WorkDir: req.WorkDir,
			Env:     req.Env,
			Memory:  req.Memory,
			CPUs:    req.CPUs,
			RepoURL: req.RepoURL,
		}

		if req.Timeout != "" {
			d, err := time.ParseDuration(req.Timeout)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid timeout: %v", err))
				return
			}
			cfg.Timeout = d
		}

		for _, ms := range req.Mounts {
			m, err := parseMount(ms)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			cfg.Mounts = append(cfg.Mounts, m)
		}

		sb, err := provider.Create(r.Context(), cfg)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := createResponse{
			SandboxID: sb.ID(),
			Status:    "created",
		}
		if req.Timeout != "" {
			resp.Timeout = req.Timeout
		}
		writeJSON(w, http.StatusCreated, resp)
	}
}

func handleListSandboxes(provider *sandbox.PodmanProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		infos, err := provider.List(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		entries := make([]sandboxEntry, 0, len(infos))
		for _, info := range infos {
			entries = append(entries, sandboxEntry{
				ID:     info.ID,
				Name:   info.Name,
				Image:  info.Image,
				Status: string(info.Status),
			})
		}
		writeJSON(w, http.StatusOK, listResponse{Sandboxes: entries})
	}
}

func handleGetStatus(provider *sandbox.PodmanProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "sandbox id is required")
			return
		}

		sb, err := provider.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		status, err := sb.Status(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, statusResponse{
			SandboxID: id,
			Status:    string(status),
		})
	}
}

func handleExecCommand(provider *sandbox.PodmanProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "sandbox id is required")
			return
		}

		var req execRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Command == "" {
			writeError(w, http.StatusBadRequest, "command is required")
			return
		}

		sb, err := provider.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		cmd := sandbox.Command{
			Cmd:  "sh",
			Args: []string{"-c", req.Command},
			Dir:  req.WorkDir,
			Env:  req.Env,
		}

		result, err := sb.Exec(r.Context(), cmd)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, execResponse{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		})
	}
}

func handleExecStream(provider *sandbox.PodmanProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "sandbox id is required")
			return
		}

		var req execRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.Command == "" {
			writeError(w, http.StatusBadRequest, "command is required")
			return
		}

		sb, err := provider.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		cmd := sandbox.Command{
			Cmd:  "sh",
			Args: []string{"-c", req.Command},
			Dir:  req.WorkDir,
			Env:  req.Env,
		}

		var mu sync.Mutex
		var output []streamLine

		exitCode, err := sb.ExecStream(r.Context(), cmd, func(stream string, data []byte) {
			mu.Lock()
			output = append(output, streamLine{Stream: stream, Data: string(data)})
			mu.Unlock()
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, execStreamResponse{
			ExitCode: exitCode,
			Output:   output,
		})
	}
}

func handleDestroySandbox(provider *sandbox.PodmanProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "sandbox id is required")
			return
		}

		sb, err := provider.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		if err := sb.Destroy(r.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, destroyResponse{
			SandboxID: id,
			Status:    "destroyed",
		})
	}
}

func handleCopyToSandbox(provider *sandbox.PodmanProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "sandbox id is required")
			return
		}

		var req copyRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.HostPath == "" || req.ContainerPath == "" {
			writeError(w, http.StatusBadRequest, "host_path and container_path are required")
			return
		}

		sb, err := provider.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		if err := sb.CopyTo(r.Context(), req.HostPath, req.ContainerPath); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, copyResponse{
			Status:        "copied",
			HostPath:      req.HostPath,
			ContainerPath: req.ContainerPath,
		})
	}
}

func handleCopyFromSandbox(provider *sandbox.PodmanProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if id == "" {
			writeError(w, http.StatusBadRequest, "sandbox id is required")
			return
		}

		var req copyRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if req.ContainerPath == "" || req.HostPath == "" {
			writeError(w, http.StatusBadRequest, "container_path and host_path are required")
			return
		}

		sb, err := provider.Get(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		if err := sb.CopyFrom(r.Context(), req.ContainerPath, req.HostPath); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, copyResponse{
			Status:        "copied",
			HostPath:      req.HostPath,
			ContainerPath: req.ContainerPath,
		})
	}
}

// --- CORS middleware ---

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Health check ---

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "apex-server",
	})
}

// --- Main ---

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	provider, err := sandbox.NewPodmanProvider()
	if err != nil {
		log.Fatalf("Failed to initialize sandbox provider: %v", err)
	}

	mux := http.NewServeMux()

	// Health
	mux.HandleFunc("GET /health", handleHealth)

	// Sandbox CRUD
	mux.HandleFunc("POST /api/sandboxes", handleCreateSandbox(provider))
	mux.HandleFunc("GET /api/sandboxes", handleListSandboxes(provider))
	mux.HandleFunc("GET /api/sandboxes/{id}/status", handleGetStatus(provider))
	mux.HandleFunc("DELETE /api/sandboxes/{id}", handleDestroySandbox(provider))

	// Execution
	mux.HandleFunc("POST /api/sandboxes/{id}/exec", handleExecCommand(provider))
	mux.HandleFunc("POST /api/sandboxes/{id}/exec/stream", handleExecStream(provider))

	// File operations
	mux.HandleFunc("POST /api/sandboxes/{id}/copy-to", handleCopyToSandbox(provider))
	mux.HandleFunc("POST /api/sandboxes/{id}/copy-from", handleCopyFromSandbox(provider))

	handler := corsMiddleware(mux)

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: readHeaderTimeout,
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		log.Println("Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("Server shutdown error: %v", err)
		}
	}()

	log.Printf("apex-server listening on :%s", port)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

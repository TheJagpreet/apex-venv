package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/apex-venv/apex-venv/sandbox"
	"github.com/pterm/pterm"
)

// errBack is returned when the user wants to go back to the main menu.
var errBack = errors.New("back")

// streamStyle is used to label streamed output lines from stderr.
var streamStderrStyle = pterm.NewStyle(pterm.FgYellow)

var (
	banner = pterm.DefaultBigText.WithLetters(
		pterm.NewLettersFromStringWithStyle("apex", pterm.NewStyle(pterm.FgCyan)),
		pterm.NewLettersFromStringWithStyle("-", pterm.NewStyle(pterm.FgWhite)),
		pterm.NewLettersFromStringWithStyle("venv", pterm.NewStyle(pterm.FgMagenta)),
	)

	headerStyle = pterm.NewStyle(pterm.FgLightCyan, pterm.Bold)
	successPrn  = pterm.Success
	errorPrn    = pterm.Error
	infoPrn     = pterm.Info
	warnPrn     = pterm.Warning
)

func main() {
	if err := checkDependencies(); err != nil {
		errorPrn.Println(err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Interactive mode: no args → show menu
	if len(os.Args) < 2 {
		runInteractive(ctx)
		return
	}

	var err error
	switch os.Args[1] {
	case "create":
		err = cmdCreate(ctx, os.Args[2:])
	case "list":
		err = cmdList(ctx)
	case "exec":
		err = cmdExec(ctx, os.Args[2:])
	case "destroy":
		err = cmdDestroy(ctx, os.Args[2:])
	case "status":
		err = cmdStatus(ctx, os.Args[2:])
	case "help", "--help", "-h":
		printUsage()
	default:
		errorPrn.Printfln("Unknown command: %s", os.Args[1])
		fmt.Println()
		printUsage()
		os.Exit(1)
	}
	if err != nil {
		errorPrn.Printfln("%v", err)
		os.Exit(1)
	}
}

// runInteractive presents an interactive menu when no arguments are given.
// It loops continuously, returning to the menu after each command.
func runInteractive(ctx context.Context) {
	_ = banner.Render()
	pterm.DefaultHeader.WithBackgroundStyle(pterm.NewStyle(pterm.BgDarkGray)).
		WithTextStyle(pterm.NewStyle(pterm.FgLightWhite)).
		Println("Sandboxed environments for agent code execution")

	commands := []string{
		"create   — Create a new sandbox",
		"list     — List all sandboxes",
		"exec     — Run a command in a sandbox",
		"status   — Show sandbox status",
		"destroy  — Destroy a sandbox",
		"help     — Show usage information",
		"exit     — Exit apex-venv",
	}

	for {
		fmt.Println()
		selected, err := pterm.DefaultInteractiveSelect.
			WithOptions(commands).
			WithDefaultText("Select a command").
			Show()
		if err != nil {
			errorPrn.Println(err)
			continue
		}

		cmd := strings.Fields(selected)[0]
		fmt.Println()

		var cmdErr error
		switch cmd {
		case "create":
			cmdErr = interactiveCreate(ctx)
		case "list":
			cmdErr = cmdList(ctx)
		case "exec":
			cmdErr = interactiveExec(ctx)
		case "status":
			cmdErr = interactiveStatus(ctx)
		case "destroy":
			cmdErr = interactiveDestroy(ctx)
		case "help":
			printUsage()
		case "exit":
			infoPrn.Println("Goodbye!")
			return
		}
		if cmdErr != nil && !errors.Is(cmdErr, errBack) {
			errorPrn.Printfln("%v", cmdErr)
		}
	}
}

// promptText shows a text input prompt; if the user types "back" or ":b" it
// returns ("", errBack) so the caller can abort and return to the menu.
func promptText(label string) (string, error) {
	raw, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText(label + pterm.Gray(" (type 'back' to return)")).
		Show()
	if err != nil {
		return "", err
	}
	v := strings.TrimSpace(raw)
	if strings.EqualFold(v, "back") || v == ":b" {
		return "", errBack
	}
	return v, nil
}

func interactiveCreate(ctx context.Context) error {
	cfg := sandbox.Config{}

	image, err := promptText("Container image (e.g. apex-venv/ubuntu)")
	if err != nil {
		return err
	}
	if image == "" {
		return fmt.Errorf("image is required")
	}
	cfg.Image = image

	name, err := promptText("Sandbox name (optional, press Enter to skip)")
	if err != nil {
		return err
	}
	cfg.Name = name

	workdir, err := promptText("Working directory (optional, press Enter to skip)")
	if err != nil {
		return err
	}
	cfg.WorkDir = workdir

	memory, err := promptText("Memory limit e.g. 512m, 2g (optional, press Enter to skip)")
	if err != nil {
		return err
	}
	cfg.Memory = memory

	cpusStr, err := promptText("CPU limit e.g. 1.5 (optional, press Enter to skip)")
	if err != nil {
		return err
	}
	if cpusStr != "" {
		var cpus float64
		if _, err := fmt.Sscanf(cpusStr, "%f", &cpus); err != nil {
			return fmt.Errorf("invalid CPU value: %s", cpusStr)
		}
		cfg.CPUs = cpus
	}

	envStr, err := promptText("Environment variables KEY=VAL,... (optional, press Enter to skip)")
	if err != nil {
		return err
	}
	if envStr != "" {
		for _, e := range strings.Split(envStr, ",") {
			e = strings.TrimSpace(e)
			if e != "" {
				cfg.Env = append(cfg.Env, e)
			}
		}
	}

	repoURL, err := promptText("Git repo URL to clone (optional, press Enter to skip)")
	if err != nil {
		return err
	}
	cfg.RepoURL = repoURL

	mountStr, err := promptText("Mounts src:dst[:ro],... (optional, press Enter to skip)")
	if err != nil {
		return err
	}
	if mountStr != "" {
		for _, ms := range strings.Split(mountStr, ",") {
			ms = strings.TrimSpace(ms)
			if ms != "" {
				m, err := parseMount(ms)
				if err != nil {
					return err
				}
				cfg.Mounts = append(cfg.Mounts, m)
			}
		}
	}

	timeoutStr, err := promptText("Timeout e.g. 30m, 2h (optional, press Enter to skip)")
	if err != nil {
		return err
	}
	if timeoutStr != "" {
		d, parseErr := time.ParseDuration(timeoutStr)
		if parseErr != nil {
			return fmt.Errorf("invalid timeout value: %s", timeoutStr)
		}
		cfg.Timeout = d
	}

	fmt.Println()
	return doCreate(ctx, cfg)
}

func interactiveExec(ctx context.Context) error {
	id, err := promptText("Sandbox ID")
	if err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("sandbox ID is required")
	}

	cmdInput, err := promptText("Command to run (e.g. ls -la /workspace)")
	if err != nil {
		return err
	}
	if cmdInput == "" {
		return fmt.Errorf("command is required")
	}

	parts := strings.Fields(cmdInput)
	fmt.Println()

	provider := newProvider()
	sb, err := provider.Get(ctx, id)
	if err != nil {
		return err
	}

	headerStyle.Println("Streaming output:")
	fmt.Println()

	exitCode, err := sb.ExecStream(ctx, sandbox.Command{
		Cmd:  parts[0],
		Args: parts[1:],
	}, func(stream string, data []byte) {
		if stream == "stderr" {
			streamStderrStyle.Printfln("[stderr] %s", string(data))
		} else {
			fmt.Println(string(data))
		}
	})
	if err != nil {
		errorPrn.Println("Execution failed")
		return err
	}

	fmt.Println()
	if exitCode == 0 {
		successPrn.Println("Command completed")
	} else {
		errorPrn.Printfln("Command exited with code %d", exitCode)
	}
	return nil
}

func interactiveStatus(ctx context.Context) error {
	id, err := promptText("Sandbox ID")
	if err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("sandbox ID is required")
	}
	fmt.Println()
	return cmdStatus(ctx, []string{id})
}

func interactiveDestroy(ctx context.Context) error {
	id, err := promptText("Sandbox ID")
	if err != nil {
		return err
	}
	if id == "" {
		return fmt.Errorf("sandbox ID is required")
	}

	confirm, _ := pterm.DefaultInteractiveConfirm.
		WithDefaultText(fmt.Sprintf("Destroy sandbox %s?", id)).
		Show()

	// Brief pause so the terminal processes any lingering input from the
	// confirm prompt before the next interactive select renders.
	time.Sleep(100 * time.Millisecond)

	if !confirm {
		warnPrn.Println("Cancelled")
		return nil
	}

	fmt.Println()
	return cmdDestroy(ctx, []string{id})
}

// checkDependencies verifies that required external tools are installed.
func checkDependencies() error {
	var missing []string

	if _, err := exec.LookPath("podman"); err != nil {
		missing = append(missing, "podman")
	}

	if _, err := exec.LookPath("go"); err != nil {
		missing = append(missing, "go")
	}

	if len(missing) == 0 {
		return nil
	}

	msg := fmt.Sprintf("required dependencies not found: %s\n\nPlease install the following before running apex-venv:\n", strings.Join(missing, ", "))
	for _, dep := range missing {
		switch dep {
		case "podman":
			msg += "  - podman: https://podman.io/docs/installation\n"
		case "go":
			msg += "  - go: https://go.dev/doc/install\n"
		}
	}
	return fmt.Errorf("%s", msg)
}

func printUsage() {
	headerStyle.Println("apex-venv — sandboxed environments for agent code execution")
	fmt.Println()

	pterm.DefaultSection.Println("Usage")
	fmt.Println("  apex-venv <command> [arguments]")
	fmt.Println("  apex-venv              (interactive mode)")
	fmt.Println()

	pterm.DefaultSection.Println("Commands")
	td := pterm.TableData{
		{"Command", "Description"},
		{"create", "Create a new sandbox"},
		{"list", "List all sandboxes"},
		{"exec", "Run a command in a sandbox"},
		{"status", "Show sandbox status"},
		{"destroy", "Destroy a sandbox"},
		{"help", "Show this help message"},
	}
	_ = pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(td).Render()
	fmt.Println()

	pterm.DefaultSection.Println("Create Flags")
	fd := pterm.TableData{
		{"Flag", "Description", "Required"},
		{"--image <image>", "Container image to use", "Yes"},
		{"--name <name>", "Human-readable sandbox name", "No"},
		{"--workdir <dir>", "Working directory inside container", "No"},
		{"--env KEY=VAL", "Environment variable (repeatable)", "No"},
		{"--mount src:dst[:ro]", "Bind mount (repeatable)", "No"},
		{"--memory <limit>", "Memory limit (e.g. 512m, 2g)", "No"},
		{"--cpus <n>", "CPU limit (e.g. 1.5)", "No"},
		{"--repo <url>", "Git repo URL to clone into the sandbox", "No"},
		{"--timeout <duration>", "Auto-destroy after duration (e.g. 30m, 2h)", "No"},
	}
	_ = pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(fd).Render()
	fmt.Println()

	pterm.DefaultSection.Println("Examples")
	exStyle := pterm.NewStyle(pterm.FgLightGreen)
	exStyle.Println("  apex-venv create --image apex-venv/ubuntu --name my-sandbox")
	exStyle.Println("  apex-venv create --image apex-venv/ubuntu --memory 512m --cpus 2")
	exStyle.Println("  apex-venv create --image apex-venv/python-3.12 --repo https://github.com/user/project.git")
	exStyle.Println("  apex-venv list")
	exStyle.Println("  apex-venv exec <sandbox-id> -- ls -la /workspace")
	exStyle.Println("  apex-venv status <sandbox-id>")
	exStyle.Println("  apex-venv destroy <sandbox-id>")
}

func newProvider() *sandbox.PodmanProvider {
	p, err := sandbox.NewPodmanProvider()
	if err != nil {
		errorPrn.Printfln("%v", err)
		os.Exit(1)
	}
	return p
}

func cmdCreate(ctx context.Context, args []string) error {
	cfg := sandbox.Config{}

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--image":
			i++
			if i >= len(args) {
				return fmt.Errorf("--image requires a value")
			}
			cfg.Image = args[i]
		case "--name":
			i++
			if i >= len(args) {
				return fmt.Errorf("--name requires a value")
			}
			cfg.Name = args[i]
		case "--workdir":
			i++
			if i >= len(args) {
				return fmt.Errorf("--workdir requires a value")
			}
			cfg.WorkDir = args[i]
		case "--env":
			i++
			if i >= len(args) {
				return fmt.Errorf("--env requires a value")
			}
			cfg.Env = append(cfg.Env, args[i])
		case "--mount":
			i++
			if i >= len(args) {
				return fmt.Errorf("--mount requires a value")
			}
			m, err := parseMount(args[i])
			if err != nil {
				return err
			}
			cfg.Mounts = append(cfg.Mounts, m)
		case "--memory":
			i++
			if i >= len(args) {
				return fmt.Errorf("--memory requires a value")
			}
			cfg.Memory = args[i]
		case "--cpus":
			i++
			if i >= len(args) {
				return fmt.Errorf("--cpus requires a value")
			}
			var cpus float64
			if _, err := fmt.Sscanf(args[i], "%f", &cpus); err != nil {
				return fmt.Errorf("invalid --cpus value: %s", args[i])
			}
			cfg.CPUs = cpus
		case "--repo":
			i++
			if i >= len(args) {
				return fmt.Errorf("--repo requires a value")
			}
			cfg.RepoURL = args[i]
		case "--timeout":
			i++
			if i >= len(args) {
				return fmt.Errorf("--timeout requires a value")
			}
			d, parseErr := time.ParseDuration(args[i])
			if parseErr != nil {
				return fmt.Errorf("invalid --timeout value: %s", args[i])
			}
			cfg.Timeout = d
		default:
			return fmt.Errorf("unknown flag: %s", args[i])
		}
	}

	// If no image provided via flags, prompt interactively
	if cfg.Image == "" {
		image, _ := pterm.DefaultInteractiveTextInput.
			WithDefaultText("Container image (required, e.g. apex-venv/ubuntu)").
			Show()
		cfg.Image = strings.TrimSpace(image)
		if cfg.Image == "" {
			return fmt.Errorf("--image is required")
		}
	}

	return doCreate(ctx, cfg)
}

func doCreate(ctx context.Context, cfg sandbox.Config) error {
	// Print config summary
	infoPrn.Printfln("Creating sandbox with image %s", pterm.LightCyan(cfg.Image))
	if cfg.Name != "" {
		infoPrn.Printfln("Name: %s", pterm.LightCyan(cfg.Name))
	}
	if cfg.Memory != "" {
		infoPrn.Printfln("Memory: %s", pterm.LightCyan(cfg.Memory))
	}
	if cfg.CPUs > 0 {
		infoPrn.Printfln("CPUs: %s", pterm.LightCyan(fmt.Sprintf("%.2f", cfg.CPUs)))
	}
	if cfg.RepoURL != "" {
		infoPrn.Printfln("Repo: %s", pterm.LightCyan(cfg.RepoURL))
	}
	if cfg.Timeout > 0 {
		infoPrn.Printfln("Timeout: %s", pterm.LightCyan(cfg.Timeout.String()))
	}
	fmt.Println()

	spinner, _ := pterm.DefaultSpinner.
		WithText("Creating sandbox...").
		WithStyle(pterm.NewStyle(pterm.FgLightCyan)).
		WithRemoveWhenDone(false).
		Start()

	provider := newProvider()
	sb, err := provider.Create(ctx, cfg)
	if err != nil {
		spinner.Fail("Failed to create sandbox")
		return err
	}

	spinner.Success("Sandbox created!")
	fmt.Println()

	id := sb.ID()
	if len(id) > 12 {
		id = id[:12]
	}
	details := fmt.Sprintf("ID:    %s\nImage: %s", pterm.LightGreen(id), pterm.LightMagenta(cfg.Image))
	if cfg.Name != "" {
		details += fmt.Sprintf("\nName:  %s", pterm.LightYellow(cfg.Name))
	}
	if cfg.Timeout > 0 {
		details += fmt.Sprintf("\nTimeout: %s", pterm.LightYellow(cfg.Timeout.String()))
	}
	panel := pterm.DefaultBox.
		WithTitle(pterm.LightCyan("Sandbox Details")).
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(pterm.FgGreen)).
		Sprint(details)
	fmt.Println(panel)
	return nil
}

func cmdList(ctx context.Context) error {
	spinner, _ := pterm.DefaultSpinner.
		WithText("Fetching sandboxes...").
		WithStyle(pterm.NewStyle(pterm.FgLightCyan)).
		Start()

	provider := newProvider()
	sandboxes, err := provider.List(ctx)
	if err != nil {
		spinner.Fail("Failed to list sandboxes")
		return err
	}

	spinner.Stop()

	if len(sandboxes) == 0 {
		warnPrn.Println("No sandboxes found")
		return nil
	}

	successPrn.Printfln("Found %d sandbox(es)", len(sandboxes))
	fmt.Println()

	td := pterm.TableData{{"ID", "NAME", "IMAGE", "STATUS"}}
	for _, s := range sandboxes {
		id := s.ID
		if len(id) > 12 {
			id = id[:12]
		}
		status := string(s.Status)
		switch s.Status {
		case sandbox.StatusRunning:
			status = pterm.LightGreen(status)
		case sandbox.StatusStopped:
			status = pterm.LightRed(status)
		default:
			status = pterm.LightYellow(status)
		}
		name := s.Name
		if name == "" {
			name = "—"
		}
		td = append(td, []string{id, name, s.Image, status})
	}
	_ = pterm.DefaultTable.WithHasHeader().WithBoxed().WithData(td).Render()
	return nil
}

func cmdExec(ctx context.Context, args []string) error {
	if len(args) < 3 {
		return fmt.Errorf("usage: apex-venv exec <sandbox-id> -- <command> [args...]")
	}

	sandboxID := args[0]

	// Find "--" separator
	dashIdx := -1
	for i, a := range args {
		if a == "--" {
			dashIdx = i
			break
		}
	}
	if dashIdx == -1 || dashIdx+1 >= len(args) {
		return fmt.Errorf("usage: apex-venv exec <sandbox-id> -- <command> [args...]")
	}

	cmdParts := args[dashIdx+1:]

	provider := newProvider()
	sb, err := provider.Get(ctx, sandboxID)
	if err != nil {
		return err
	}

	infoPrn.Printfln("Running: %s", strings.Join(cmdParts, " "))
	fmt.Println()

	exitCode, err := sb.ExecStream(ctx, sandbox.Command{
		Cmd:  cmdParts[0],
		Args: cmdParts[1:],
	}, func(stream string, data []byte) {
		if stream == "stderr" {
			fmt.Fprintf(os.Stderr, "%s\n", string(data))
		} else {
			fmt.Println(string(data))
		}
	})
	if err != nil {
		errorPrn.Println("Execution failed")
		return err
	}

	fmt.Println()
	if exitCode == 0 {
		successPrn.Printfln("Command completed (exit code %d)", exitCode)
	} else {
		errorPrn.Printfln("Command exited with code %d", exitCode)
		return fmt.Errorf("command exited with code %d", exitCode)
	}
	return nil
}

func cmdStatus(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: apex-venv status <sandbox-id>")
	}

	provider := newProvider()
	sb, err := provider.Get(ctx, args[0])
	if err != nil {
		return err
	}

	status, err := sb.Status(ctx)
	if err != nil {
		return err
	}

	var statusColored string
	switch status {
	case sandbox.StatusRunning:
		statusColored = pterm.LightGreen(string(status))
	case sandbox.StatusStopped:
		statusColored = pterm.LightRed(string(status))
	default:
		statusColored = pterm.LightYellow(string(status))
	}

	id := args[0]
	if len(id) > 12 {
		id = id[:12]
	}
	panel := pterm.DefaultBox.
		WithTitle(pterm.LightCyan("Sandbox Status")).
		WithTitleTopCenter().
		WithBoxStyle(pterm.NewStyle(pterm.FgCyan)).
		Sprint(
			fmt.Sprintf("ID:     %s\nStatus: %s", pterm.LightWhite(id), statusColored),
		)
	fmt.Println(panel)
	return nil
}

func cmdDestroy(ctx context.Context, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: apex-venv destroy <sandbox-id>")
	}

	provider := newProvider()
	sb, err := provider.Get(ctx, args[0])
	if err != nil {
		return err
	}

	spinner, _ := pterm.DefaultSpinner.
		WithText(fmt.Sprintf("Destroying sandbox %s...", args[0])).
		WithStyle(pterm.NewStyle(pterm.FgLightRed)).
		Start()
	// Brief pause so spinner is visible
	time.Sleep(300 * time.Millisecond)

	if err := sb.Destroy(ctx); err != nil {
		spinner.Fail("Failed to destroy sandbox")
		return err
	}

	spinner.Success(fmt.Sprintf("Sandbox %s destroyed", args[0]))
	return nil
}

func parseMount(s string) (sandbox.Mount, error) {
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return sandbox.Mount{}, fmt.Errorf("invalid mount format %q, expected src:dst[:ro]", s)
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

func fatal(msg string) {
	errorPrn.Println(msg)
	os.Exit(1)
}

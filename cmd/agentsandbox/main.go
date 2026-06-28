// Package main is the CLI entrypoint for AgentSandbox.
//
// When a developer or agent executes the "agentsandbox run" command, this
// package orchestrates the primary sandbox lifecycle:
//
//	CLI Parsing → Backend Selection → Policy Enforcement → Sandbox Execution → Diff Capture → Tracing
//
// The CLI serves purely as the orchestrator and depends entirely on the internal
// packages for business logic, maintaining a clear separation of concerns across
// the codebase architecture.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/ritikraj2425/agentsandbox/internal/approvals"
	"github.com/ritikraj2425/agentsandbox/internal/color"
	"github.com/ritikraj2425/agentsandbox/internal/fsdiff"
	"github.com/ritikraj2425/agentsandbox/internal/gateway"
	"github.com/ritikraj2425/agentsandbox/internal/policy"
	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
	"strconv"

	// Import backend packages so their init() functions register with the
	// runtime registry. Adding a new backend is as simple as adding an
	// import line here — the CLI does not need any other changes.
	dockerrt "github.com/ritikraj2425/agentsandbox/runtimes/docker"
	firecrackerrt "github.com/ritikraj2425/agentsandbox/runtimes/firecracker"
	gvisorrt "github.com/ritikraj2425/agentsandbox/runtimes/gvisor"
	localrt "github.com/ritikraj2425/agentsandbox/runtimes/local"
	browserrt "github.com/ritikraj2425/agentsandbox/runtimes/browser"
)

const version = "0.6.0"

func main() {
	if len(os.Args) < 2 || os.Args[1] == "--help" || os.Args[1] == "-h" {
		printUsage()
		os.Exit(0)
	}

	switch os.Args[1] {
	case "run":
		cmdRun(os.Args[2:])
	case "serve":
		cmdServe(os.Args[2:])
	case "version":
		fmt.Printf("agentsandbox %s\n", version)
	default:
		fmt.Fprintf(os.Stderr, "%s Unknown command: %s\n\n", color.Red("✗"), os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Printf(`%s - Open-source Go runtime for safely running AI agent actions

%s
  agentsandbox <command> [flags]

%s
  run       Run a shell command inside the sandbox
  serve     Start the Multi-Tenant API Gateway server
  version   Print the AgentSandbox version

%s
  agentsandbox run "echo hello"
  agentsandbox serve --port 8080 --max-sessions 1000 --auth-key secret
  agentsandbox run --backend local --policy policy.yaml "go test ./..."
  agentsandbox run --json "ls -la"

%s
  -h, --help   Show this help message

%s https://github.com/ritikraj2425/agentsandbox
`,
		color.BoldCyan("AgentSandbox"),
		color.Bold("Usage:"),
		color.Bold("Commands:"),
		color.Bold("Examples:"),
		color.Bold("Flags:"),
		color.Dim("Documentation:"),
	)
}

// cmdRun handles the "agentsandbox run <command>" subcommand.
//
// The full execution flow:
//  1. Parse CLI arguments (command, --policy, --backend, --json, --yes, --non-interactive).
//  2. Create a standard protocol.Action from the command string.
//  3. Initialize the trace logger.
//  4. Resolve and construct the requested runtime backend.
//  5. If --policy is set, enforce command policy rules.
//  6. Snapshot the workspace (pre-execution).
//  7. Execute via the runtime backend.
//  8. Snapshot the workspace (post-execution) and compute filesystem diff.
//  9. Write report and display result.
func cmdRun(args []string) {
	command := ""
	policyPath := ""
	backendName := "local"
	dockerImage := ""
	cpus := ""
	memory := ""
	kernelPath := ""
	rootfsPath := ""
	showJSON := false
	autoApprove := false
	nonInteractive := false

	// ── Argument parsing ─────────────────────────────────────────────────
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			fmt.Printf(`%s

%s
  agentsandbox run [flags] "<command>"

%s
  command   The shell command to execute (required, must be quoted)

%s
  --backend <name>  Select execution backend: local, docker, gvisor, firecracker (default: local)
  --image <image>   Docker/gVisor image to use
  --cpus <limit>    CPU limit for docker/gvisor (e.g. 1.5)
  --memory <limit>  Memory limit for docker/gvisor (e.g. 2gb)
  --kernel <path>   Path to vmlinux kernel image (firecracker backend)
  --rootfs <path>   Path to root filesystem ext4 image (firecracker backend)
  --policy <file>   Load a YAML policy file to enforce command rules
  --json            Output the full Observation as JSON
  --yes             Automatically approve commands that require approval
  --non-interactive Automatically deny commands that require approval
  -h, --help        Show this help message

%s
  agentsandbox run "echo hello"
  agentsandbox run --backend local "echo hello"
  agentsandbox run --backend docker --image golang:1.26 "go test ./..."
  agentsandbox run --backend gvisor --cpus 1.5 --memory 2gb "go test ./..."
  agentsandbox run --backend firecracker --kernel vmlinux --rootfs rootfs.ext4 "echo hello"
  agentsandbox run --policy examples/policy.yaml "go test ./..."
  agentsandbox run --json "ls -la"
`,
				color.Bold("Run a shell command inside the sandbox"),
				color.Bold("Usage:"),
				color.Bold("Arguments:"),
				color.Bold("Flags:"),
				color.Bold("Examples:"),
			)
			return

		case "--json":
			showJSON = true

		case "--yes":
			autoApprove = true

		case "--non-interactive":
			nonInteractive = true

		case "--backend":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s --backend requires a name argument\n", color.Red("✗"))
				fmt.Fprintf(os.Stderr, "%s agentsandbox run --backend local \"command\"\n", color.Dim("Usage:"))
				os.Exit(1)
			}
			i++
			backendName = args[i]

		case "--image":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s --image requires a Docker image name\n", color.Red("✗"))
				fmt.Fprintf(os.Stderr, "%s agentsandbox run --backend docker --image golang:1.26 \"command\"\n", color.Dim("Usage:"))
				os.Exit(1)
			}
			i++
			dockerImage = args[i]

		case "--cpus":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s --cpus requires a value\n", color.Red("✗"))
				os.Exit(1)
			}
			i++
			cpus = args[i]

		case "--memory":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s --memory requires a value\n", color.Red("✗"))
				os.Exit(1)
			}
			i++
			memory = args[i]

		case "--kernel":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s --kernel requires a path to vmlinux\n", color.Red("✗"))
				os.Exit(1)
			}
			i++
			kernelPath = args[i]

		case "--rootfs":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s --rootfs requires a path to rootfs.ext4\n", color.Red("✗"))
				os.Exit(1)
			}
			i++
			rootfsPath = args[i]

		case "--policy":
			if i+1 >= len(args) {
				fmt.Fprintf(os.Stderr, "%s --policy requires a file path argument\n", color.Red("✗"))
				fmt.Fprintf(os.Stderr, "%s agentsandbox run --policy policy.yaml \"command\"\n", color.Dim("Usage:"))
				os.Exit(1)
			}
			i++
			policyPath = args[i]

		default:
			command = args[i]
		}
	}

	// Validate that a command was provided.
	if command == "" {
		fmt.Fprintf(os.Stderr, "%s No command provided\n", color.Red("✗"))
		fmt.Fprintf(os.Stderr, "%s agentsandbox run \"<command>\"\n", color.Dim("Usage:"))
		os.Exit(1)
	}

	// ── Step 1: Create the Action ────────────────────────────────────────
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
		"command": command,
	})

	// ── Step 2: Set up working directory ─────────────────────────────────
	workDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s Could not determine working directory: %s\n", color.Red("✗"), err)
		os.Exit(1)
	}

	// ── Step 3: Initialize the trace logger ──────────────────────────────
	logger, logErr := trace.NewRunLogger(workDir)
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "%s Could not initialize trace logger: %s\n",
			color.Yellow("⚠"), logErr)
	}
	if logger != nil {
		defer logger.Close()

		logger.LogEvent(trace.EventTypeActionReceived, "Received command from CLI", map[string]interface{}{
			"action_id":   action.ID,
			"action_type": string(action.Type),
			"command":     command,
			"policy_file": policyPath,
			"backend":     backendName,
		})
	}

	// ── Step 4: Resolve runtime backend ──────────────────────────────────
	//
	// We use a factory approach: the local runtime package self-registers
	// via init(). However, to inject the logger into the runtime, we use
	// direct construction for the local backend. Future backends (docker,
	// gvisor) will use the registry for fully decoupled instantiation.
	var rt runtime.Runtime

	switch backendName {
	case "local":
		rt = localrt.New(workDir, logger)
	case "docker":
		var rtErr error
		rt, rtErr = dockerrt.New(workDir, dockerImage, logger)
		if rtErr != nil {
			fmt.Fprintf(os.Stderr, "%s Docker backend unavailable: %s\n", color.Red("✗"), rtErr)
			fmt.Fprintf(os.Stderr, "%s Make sure Docker Desktop is running\n", color.Dim("Hint:"))
			os.Exit(1)
		}
	case "gvisor":
		var rtErr error
		rt, rtErr = gvisorrt.New(workDir, dockerImage, cpus, memory, logger)
		if rtErr != nil {
			fmt.Fprintf(os.Stderr, "%s gVisor backend unavailable: %s\n", color.Red("✗"), rtErr)
			fmt.Fprintf(os.Stderr, "%s Make sure runsc is configured in Docker\n", color.Dim("Hint:"))
			os.Exit(1)
		}
	case "firecracker":
		var rtErr error
		rt, rtErr = firecrackerrt.New(firecrackerrt.Config{
			WorkDir:    workDir,
			KernelPath: kernelPath,
			RootFSPath: rootfsPath,
			Logger:     logger,
		})
		if rtErr != nil {
			fmt.Fprintf(os.Stderr, "%s Firecracker backend unavailable: %s\n", color.Red("✗"), rtErr)
			fmt.Fprintf(os.Stderr, "%s Firecracker requires Linux with KVM support\n", color.Dim("Hint:"))
			os.Exit(1)
		}
	case "browser":
		var rtErr error
		rt, rtErr = browserrt.New(browserrt.Config{
			WorkDir: workDir,
			Logger:  logger,
		})
		if rtErr != nil {
			fmt.Fprintf(os.Stderr, "%s Browser backend unavailable: %s\n", color.Red("✗"), rtErr)
			os.Exit(1)
		}
	default:
		// Check the registry for dynamically registered backends.
		factory, exists := runtime.Registry[backendName]
		if !exists {
			fmt.Fprintf(os.Stderr, "%s Unknown backend: %q\n", color.Red("✗"), backendName)
			fmt.Fprintf(os.Stderr, "%s Available backends: local, docker, gvisor, firecracker\n", color.Dim("Hint:"))
			os.Exit(1)
		}
		var rtErr error
		rt, rtErr = factory(workDir)
		if rtErr != nil {
			fmt.Fprintf(os.Stderr, "%s Failed to initialize backend %q: %s\n", color.Red("✗"), backendName, rtErr)
			os.Exit(1)
		}
	}

	if !showJSON {
		fmt.Printf("  %s  %s\n",
			color.Cyan("Backend:  "),
			color.Dim(rt.Name()),
		)
	}

	// ── Step 5: Policy check ─────────────────────────────────────────────
	if policyPath != "" {
		cmdPolicy, policyErr := policy.LoadCommandPolicyFromFile(policyPath)
		if policyErr != nil {
			fmt.Fprintf(os.Stderr, "%s Failed to load policy: %s\n", color.Red("✗"), policyErr)
			os.Exit(1)
		}

		decision := cmdPolicy.CheckCommand(command)

		// Log the policy decision to the trace.
		if logger != nil {
			logger.LogEvent(trace.EventTypePolicyCheck, "Policy evaluated", map[string]interface{}{
				"policy_name":  cmdPolicy.Name,
				"command":      command,
				"allowed":      decision.Allowed,
				"effect":       decision.Effect,
				"matched_rule": decision.MatchedRule,
				"reason":       decision.Reason,
			})
		}

		if !decision.Allowed {
			if decision.Effect == "require_approval" {
				if logger != nil {
					logger.LogEvent(trace.EventTypeApprovalRequested, "Approval requested", nil)
				}
				req := approvals.Request{
					Command: command,
					Reason:  decision.Reason,
				}
				cfg := approvals.Config{
					AutoApprove:    autoApprove,
					NonInteractive: nonInteractive,
					Reader:         os.Stdin,
					Writer:         os.Stdout,
				}

				approved := approvals.Ask(req, cfg)
				if logger != nil {
					logger.LogEvent(trace.EventTypeApprovalDecision, "Approval decision", map[string]interface{}{
						"approved": approved,
					})
				}

				if approved {
					decision.Allowed = true
				} else {
					decision.Reason = "denied by user during interactive approval"
				}
			}
		}

		if !decision.Allowed {
			// Build a "denied" Observation without executing anything.
			obs := protocol.NewObservation(action.ID)
			obs.Command = command
			obs.Status = protocol.ObsStatusDenied
			obs.ExitCode = -1
			obs.Error = decision.Reason
			obs.DurationMs = 0
			obs.Backend = rt.Name()

			if logger != nil {
				logger.WriteReport(obs)
			}

			if showJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(obs)
			} else {
				printObservation(obs, logger, cmdPolicy.Name)
			}

			os.Exit(1)
		}

		// Policy allowed the command — print confirmation and continue.
		if !showJSON {
			fmt.Printf("  %s  %s %s\n",
				color.Cyan("Policy:   "),
				color.Green("✓"),
				color.Dim(fmt.Sprintf("allowed by %q (matched: %s)", cmdPolicy.Name, decision.MatchedRule)),
			)
		}
	}

	// ── Step 6: Pre-execution Snapshot ────────────────────────────────────
	var beforeSnapshot fsdiff.Snapshot
	ignores := []string{".git", ".agentsandbox"}
	if logger != nil {
		beforeSnapshot, _ = fsdiff.TakeSnapshot(workDir, ignores)
	}

	// ── Step 7: Execute the command via the runtime backend ──────────────
	obs, _ := rt.Run(context.Background(), action)

	// ── Step 8: Post-execution Snapshot and Diff ─────────────────────────
	if logger != nil {
		afterSnapshot, _ := fsdiff.TakeSnapshot(workDir, ignores)
		diff := fsdiff.Compare(action.ID, beforeSnapshot, afterSnapshot)

		obs.FilesChanged = append(diff.FilesAdded, diff.FilesModified...)
		obs.FilesDeleted = diff.FilesDeleted

		logger.WriteDiff(diff)
	}

	// ── Step 9: Write report and display result ──────────────────────────
	if logger != nil {
		logger.WriteReport(obs)
	}

	if showJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(obs)
	} else {
		policyName := ""
		if policyPath != "" {
			if p, err := policy.LoadCommandPolicyFromFile(policyPath); err == nil {
				policyName = p.Name
			}
		}
		printObservation(obs, logger, policyName)
	}

	if obs.ExitCode > 0 {
		os.Exit(obs.ExitCode)
	} else if obs.Status != protocol.ObsStatusCompleted {
		os.Exit(1)
	}
}

// printObservation displays the Observation as a clean, colored summary.
func printObservation(obs protocol.Observation, logger *trace.RunLogger, policyName string) {
	fmt.Println()
	fmt.Println(color.BoldCyan("━━━ AgentSandbox Run ━━━"))

	fmt.Printf("  %s  %s\n", color.Cyan("Action ID:"), obs.ActionID)
	fmt.Printf("  %s  %s\n", color.Cyan("Command:  "), obs.Command)

	if obs.Backend != "" {
		fmt.Printf("  %s  %s\n", color.Cyan("Backend:  "), color.Dim(obs.Backend))
	}

	// Status with color-coded icon.
	statusText := string(obs.Status)
	statusColorFn := color.StatusColor(statusText)
	statusIcon := "✓"
	switch obs.Status {
	case protocol.ObsStatusFailed:
		statusIcon = "✗"
	case protocol.ObsStatusTimeout:
		statusIcon = "⏱"
	case protocol.ObsStatusDenied:
		statusIcon = "⊘"
	}
	fmt.Printf("  %s  %s %s\n", color.Cyan("Status:   "), statusColorFn(statusIcon), statusColorFn(statusText))

	// Exit code: green for 0, red for everything else.
	exitCodeStr := fmt.Sprintf("%d", obs.ExitCode)
	if obs.ExitCode == 0 {
		exitCodeStr = color.Green(exitCodeStr)
	} else {
		exitCodeStr = color.Red(exitCodeStr)
	}
	fmt.Printf("  %s  %s\n", color.Cyan("Exit Code:"), exitCodeStr)

	fmt.Printf("  %s  %dms\n", color.Cyan("Duration: "), obs.DurationMs)

	// Show which policy was active (if any).
	if policyName != "" {
		fmt.Printf("  %s  %s\n", color.Cyan("Policy:   "), color.Dim(policyName))
	}

	if obs.Error != "" {
		fmt.Printf("  %s  %s\n", color.Cyan("Error:    "), color.Red(obs.Error))
	}

	if len(obs.FilesChanged) > 0 {
		fmt.Printf("  %s  %s\n", color.Cyan("Changed:  "), color.Green(fmt.Sprintf("%d file(s) modified/added", len(obs.FilesChanged))))
		for _, f := range obs.FilesChanged {
			fmt.Printf("    %s %s\n", color.Green("+"), f)
		}
	}
	if len(obs.FilesDeleted) > 0 {
		fmt.Printf("  %s  %s\n", color.Cyan("Deleted:  "), color.Red(fmt.Sprintf("%d file(s) removed", len(obs.FilesDeleted))))
		for _, f := range obs.FilesDeleted {
			fmt.Printf("    %s %s\n", color.Red("-"), f)
		}
	}

	if logger != nil {
		fmt.Printf("  %s  %s\n", color.Cyan("Run Dir:  "), color.Dim(logger.RunDir))
	}

	if obs.StdoutSummary != "" {
		fmt.Println()
		fmt.Println(color.BoldCyan("━━━ stdout ━━━"))
		fmt.Print(obs.StdoutSummary)
	}

	if obs.StderrSummary != "" {
		fmt.Println()
		fmt.Println(color.BoldCyan("━━━ stderr ━━━"))
		fmt.Print(color.Yellow(obs.StderrSummary))
	}

	fmt.Println()
}

func cmdServe(args []string) {
	port := 8080
	maxSessions := 1000
	authKey := ""

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--help", "-h":
			fmt.Printf(`Start the Multi-Tenant API Gateway server

Usage:
  agentsandbox serve [flags]

Flags:
  --port <number>         Port to listen on (default: 8080)
  --max-sessions <number> Maximum concurrent virtual sessions (default: 1000)
  --auth-key <string>     Required Bearer token for API authentication
  -h, --help              Show this help message
`)
			return
		case "--port":
			if i+1 < len(args) {
				i++
				p, _ := strconv.Atoi(args[i])
				if p > 0 {
					port = p
				}
			}
		case "--max-sessions":
			if i+1 < len(args) {
				i++
				m, _ := strconv.Atoi(args[i])
				if m > 0 {
					maxSessions = m
				}
			}
		case "--auth-key":
			if i+1 < len(args) {
				i++
				authKey = args[i]
			}
		}
	}

	if authKey == "" {
		fmt.Fprintf(os.Stderr, "%s --auth-key is required\n", color.Red("✗"))
		os.Exit(1)
	}

	server := gateway.NewServer(port, maxSessions, authKey)
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server failed: %s\n", err)
		os.Exit(1)
	}
}

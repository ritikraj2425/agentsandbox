// Package runner provides runtime backends for executing agent actions.
//
// The local runner executes actions directly on the host machine using
// os/exec. It is the simplest backend and provides no isolation — the
// command has full access to the host filesystem, network, and environment.
//
// Why this package exists:
// The runner is the "execution runtime" from the manifesto's core loop:
//
//	Agent proposes Action → Policy checks → Runner executes → Observation returned
//
// This package handles step 3: actually running the command and capturing
// everything that happened (stdout, stderr, exit code, duration).
//
// Phase 2 update:
// RunCommand now accepts an optional *trace.RunLogger. When provided,
// the runner logs process.started and process.finished events, and writes
// stdout/stderr to the logger's log files on disk.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/actions"
	"github.com/ritikraj2425/agentsandbox/internal/policy"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// Runner is the interface all runtime backends must implement.
//
// Why an interface?
// We will have multiple backends: local, Docker, gVisor, Firecracker.
// All of them accept the same Action and return the same Observation.
// The CLI and API gateway don't need to know which backend is running —
// they just call Run() and get back a standard result.
type Runner interface {
	// Run executes the given action and returns a trace of events.
	Run(action *actions.Action, pol *policy.Policy) ([]trace.TraceEvent, error)
}

// DefaultTimeout is how long a command is allowed to run before being killed.
//
// 60 seconds is generous for most agent actions (running tests, linting,
// reading files). If an agent needs longer, the timeout should be
// configurable via CLI flags (added in a future phase).
//
// Why use timeouts at all?
// AI agents can accidentally run infinite loops, blocking builds, or
// commands that hang forever waiting for input. The timeout prevents
// a single action from consuming resources indefinitely.
const DefaultTimeout = 60 * time.Second

// LocalRunner executes actions directly on the host OS.
// It is the simplest runner and provides no isolation.
type LocalRunner struct {
	// WorkDir is the working directory for command execution.
	// Commands run relative to this directory, just like if you
	// opened a terminal and cd'd into it.
	WorkDir string

	// Timeout overrides the default command timeout.
	// Zero means use DefaultTimeout.
	Timeout time.Duration
}

// NewLocalRunner creates a new LocalRunner with the given working directory.
func NewLocalRunner(workDir string) *LocalRunner {
	return &LocalRunner{WorkDir: workDir}
}

// Run executes the action locally and returns trace events.
// This method still accepts the old internal Action type for backward
// compatibility with existing tests. The CLI now uses RunCommand()
// directly for shell execution.
func (r *LocalRunner) Run(action *actions.Action, pol *policy.Policy) ([]trace.TraceEvent, error) {
	var events []trace.TraceEvent

	// Record start event.
	events = append(events, trace.NewTraceEvent(
		trace.EventTypeActionStart,
		fmt.Sprintf("Starting action: %s (%s)", action.Name, action.Type),
	))

	// Check policy.
	if pol != nil {
		allowed, reason := pol.Evaluate(action)
		events = append(events, trace.NewTraceEvent(
			trace.EventTypePolicyCheck,
			fmt.Sprintf("Policy check: allowed=%v reason=%s", allowed, reason),
		))
		if !allowed {
			action.Fail(fmt.Sprintf("denied by policy: %s", reason))
			events = append(events, trace.NewTraceEvent(
				trace.EventTypeActionEnd,
				fmt.Sprintf("Action denied: %s", reason),
			))
			return events, fmt.Errorf("action denied by policy: %s", reason)
		}
	}

	// If the action is a shell command, execute it for real.
	if action.Type == actions.ActionTypeShell {
		cmd, ok := action.Parameters["command"].(string)
		if ok && cmd != "" {
			obs := r.RunCommand(context.Background(), protocol.NewAction(
				protocol.ActionTypeShellRun,
				map[string]interface{}{"command": cmd},
			), nil) // nil logger — legacy Run() doesn't use disk logging.
			events = append(events, trace.NewTraceEvent(
				trace.EventTypeActionEnd,
				fmt.Sprintf("Command finished: exit_code=%d duration=%dms", obs.ExitCode, obs.DurationMs),
			))
			if obs.ExitCode != 0 {
				action.Fail(fmt.Sprintf("command exited with code %d", obs.ExitCode))
				return events, fmt.Errorf("command exited with code %d", obs.ExitCode)
			}
			action.Complete()
			return events, nil
		}
	}

	// Non-shell actions or missing command: placeholder behavior.
	action.Status = actions.ActionStatusRunning
	events = append(events, trace.NewTraceEvent(
		trace.EventTypeActionEnd,
		fmt.Sprintf("Action completed (placeholder): %s", action.Name),
	))
	action.Complete()

	return events, nil
}

// RunCommand is the real execution engine for shell commands.
//
// It takes a protocol.Action (the standard format) and returns a
// protocol.Observation (the standard result).
//
// Phase 2 change: the logger parameter is optional. When non-nil,
// RunCommand will:
//   - Log "process.started" before executing the command.
//   - Log "process.finished" after the command exits.
//   - Write full stdout to stdout.log via the logger.
//   - Write full stderr to stderr.log via the logger.
//
// When logger is nil, behavior is identical to Phase 1 (no disk logging).
//
// How it works:
//  1. Build the OS command using /bin/sh -c (Unix) or cmd /C (Windows).
//  2. Set the working directory.
//  3. Attach timeout via context.WithTimeout.
//  4. Capture stdout and stderr into byte buffers.
//  5. Run the command and wait for it to finish (or timeout).
//  6. Package everything into an Observation.
//
// Why /bin/sh -c instead of running the binary directly?
// Agent commands are full shell expressions like "go test ./..." or
// "cat file.txt | grep error". The shell interprets pipes, globs,
// and redirections. Without sh -c, we'd need to parse shell syntax
// ourselves, which is complex and error-prone.
func (r *LocalRunner) RunCommand(ctx context.Context, action protocol.Action, logger *trace.RunLogger) protocol.Observation {
	obs := protocol.NewObservation(action.ID)
	obs.Command = action.Command()

	// If no command was provided, return an error observation immediately.
	if obs.Command == "" {
		obs.Status = protocol.ObsStatusFailed
		obs.Error = "no command specified in action parameters"
		return obs
	}

	// Determine timeout: use the runner's configured timeout, or fall back
	// to the default 60 seconds.
	timeout := r.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	// context.WithTimeout creates a derived context that automatically
	// sends a cancellation signal after the timeout duration.
	// When the context is cancelled, Go's exec package kills the process.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel() // Always clean up the context timer to avoid leaks.

	// Build the OS-specific shell command.
	//
	// On Unix (Linux/macOS): /bin/sh -c "echo hello"
	//   - /bin/sh is the POSIX shell, available on every Unix system.
	//   - -c tells it to read the command from the next argument string.
	//
	// On Windows: cmd /C "echo hello"
	//   - cmd.exe is the Windows command interpreter.
	//   - /C tells it to execute the command and then terminate.
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", obs.Command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", obs.Command)
	}

	// Set the working directory for the command.
	// This is equivalent to cd'ing into the directory before running the command.
	cmd.Dir = r.WorkDir

	// Capture stdout and stderr into separate in-memory buffers.
	//
	// Why bytes.Buffer instead of writing directly to files?
	// 1. We need the output in memory to build the Observation summary.
	// 2. Writing to disk files is handled by the RunLogger (if provided).
	//    Separation of concerns: the runner captures, the logger persists.
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// --- Phase 2: Log process.started ---
	// This event marks the moment we hand off to the OS to actually run
	// the command. The time between "action.received" and "process.started"
	// tells us how long setup (policy checks, etc.) took.
	if logger != nil {
		logger.LogEvent(trace.EventTypeProcessStarted, "Process started", map[string]interface{}{
			"command":     obs.Command,
			"working_dir": r.WorkDir,
		})
	}

	// Record the start time to calculate duration.
	startTime := time.Now()

	// cmd.Run() does three things:
	//   1. Starts the process (fork + exec on Unix).
	//   2. Waits for it to finish.
	//   3. Returns an error if the exit code is non-zero.
	err := cmd.Run()

	// Calculate how long the command took.
	obs.DurationMs = time.Since(startTime).Milliseconds()

	// Get the raw stdout and stderr as strings.
	// We need the raw (untrimmed) versions for the log files,
	// and the truncated versions for the Observation.
	rawStdout := stdoutBuf.String()
	rawStderr := stderrBuf.String()

	// Build the observation from the captured output.
	obs.StdoutSummary = protocol.TruncateOutput(rawStdout)
	obs.StderrSummary = protocol.TruncateOutput(rawStderr)

	if err != nil {
		// Determine what went wrong.
		if ctx.Err() == context.DeadlineExceeded {
			// The context timeout fired, meaning the command ran too long.
			obs.Status = protocol.ObsStatusTimeout
			obs.Error = fmt.Sprintf("command timed out after %s", timeout)
			obs.ExitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			// The command ran but exited with a non-zero code.
			// This is normal — it means the tests failed, the build broke, etc.
			obs.Status = protocol.ObsStatusFailed
			obs.ExitCode = exitErr.ExitCode()
			obs.Error = fmt.Sprintf("command exited with code %d", obs.ExitCode)
		} else {
			// The command could not be started at all.
			// This happens when the binary doesn't exist (e.g., "invalid-command-xyz").
			obs.Status = protocol.ObsStatusFailed
			obs.ExitCode = -1
			obs.Error = fmt.Sprintf("failed to execute command: %s", err.Error())
		}
	} else {
		// The command ran successfully with exit code 0.
		obs.Status = protocol.ObsStatusCompleted
		obs.ExitCode = 0
	}

	// --- Phase 2: Log process.finished and write output files ---
	// These operations are all on the "observability" path, not the
	// critical execution path. If any of them fail, the command's result
	// is still returned correctly.
	if logger != nil {
		logger.LogEvent(trace.EventTypeProcessFinished, "Process finished", map[string]interface{}{
			"exit_code":   obs.ExitCode,
			"duration_ms": obs.DurationMs,
			"status":      string(obs.Status),
		})

		// Write the full (untrimmed) stdout and stderr to disk.
		// The Observation only contains the truncated summary for the agent,
		// but the full output is preserved in the run directory for debugging.
		logger.WriteStdout(rawStdout)
		logger.WriteStderr(rawStderr)
	}

	return obs
}

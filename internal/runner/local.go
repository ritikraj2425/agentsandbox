// Package runner provides runtime backends for executing agent actions.
//
// The local runner executes actions directly on the host machine using
// os/exec. It provides a lightweight execution layer without sandboxing,
// meaning commands have full access to the host filesystem, network,
// and environment.
//
// The Runner interface standardizes action execution across different
// backends (e.g., local, Docker, gVisor), allowing the core sandbox
// loop to remain backend-agnostic.
//
// When instantiated with a RunLogger, the runner automatically emits
// lifecycle events and persists output streams for auditing and replay.
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

// Runner defines the standard execution interface for all backends.
//
// Implementing backends (local, Docker, gVisor) must accept a standardized
// Action and return a consistent Observation, decoupling the execution
// environment from the policy and API layers.
type Runner interface {
	// Run executes the given action and returns a trace of events.
	Run(action *actions.Action, pol *policy.Policy) ([]trace.TraceEvent, error)
}

// DefaultTimeout specifies the maximum duration a command can execute
// before being forcefully terminated.
//
// Setting a strict upper bound prevents runaway processes, infinite loops,
// or hanging network requests initiated by the agent from exhausting
// system resources indefinitely.
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

// RunCommand executes a shell command and returns the resulting Observation.
//
// If a logger is provided, it emits lifecycle events and persists output streams.
//
// Commands are executed via the host's native shell interpreter (`/bin/sh -c` on Unix,
// `cmd /C` on Windows) to naturally support pipes, globs, and environment variables
// without requiring complex manual parsing.
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

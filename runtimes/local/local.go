// Package local implements the local (no-isolation) runtime backend.
//
// This is the default and simplest backend: commands execute directly on the
// host OS via /bin/sh (Unix) or cmd.exe (Windows). No containerization,
// no filesystem isolation, no network restrictions.
//
// Use this backend for development, trusted agent workflows, and environments
// where host-level access is acceptable. For production agent deployments,
// prefer the Docker or gVisor backends which provide process and filesystem
// isolation boundaries.
package local

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	goruntime "runtime"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// backendName is the canonical identifier for this runtime.
const backendName = "local"

// DefaultTimeout specifies the maximum duration a command can execute
// before being forcefully terminated.
const DefaultTimeout = 60 * time.Second

func init() {
	// Self-register with the runtime registry so the CLI can discover
	// this backend via the --backend flag without hard-coded imports.
	runtime.Register(backendName, func(workDir string) (runtime.Runtime, error) {
		return New(workDir, nil), nil
	})
}

// Runtime executes actions directly on the host operating system.
// It satisfies the runtime.Runtime interface.
type Runtime struct {
	// workDir is the working directory for command execution.
	workDir string

	// timeout overrides the default command timeout. Zero means use DefaultTimeout.
	timeout time.Duration

	// logger is the optional trace logger for persisting execution events
	// and output streams. When nil, no disk logging occurs.
	logger *trace.RunLogger
}

// New creates a new local Runtime with the given working directory and
// optional trace logger. Pass nil for logger if disk logging is not needed.
func New(workDir string, logger *trace.RunLogger) *Runtime {
	return &Runtime{
		workDir: workDir,
		logger:  logger,
	}
}

// SetTimeout overrides the default command execution timeout.
func (r *Runtime) SetTimeout(d time.Duration) {
	r.timeout = d
}

// Name returns the canonical backend identifier.
func (r *Runtime) Name() string {
	return backendName
}

// Run executes a shell command on the host OS and returns the resulting Observation.
//
// This method implements the runtime.Runtime interface. It constructs the
// platform-appropriate shell invocation, captures stdout/stderr, applies
// timeout enforcement, and emits trace events when a logger is configured.
func (r *Runtime) Run(ctx context.Context, action protocol.Action) (protocol.Observation, error) {
	obs := protocol.NewObservation(action.ID)
	obs.Command = action.Command()
	obs.Backend = backendName

	// Validate that a command was provided.
	if obs.Command == "" {
		obs.Status = protocol.ObsStatusFailed
		obs.Error = "no command specified in action parameters"
		return obs, fmt.Errorf("no command specified in action parameters")
	}

	// Resolve timeout.
	timeout := r.timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	// Apply timeout to the execution context.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the platform-specific shell command.
	var cmd *exec.Cmd
	if goruntime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", obs.Command)
	} else {
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", obs.Command)
	}

	cmd.Dir = r.workDir

	// Capture stdout and stderr into in-memory buffers.
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Emit process.started trace event.
	if r.logger != nil {
		r.logger.LogEvent(trace.EventTypeProcessStarted, "Process started", map[string]interface{}{
			"command":     obs.Command,
			"working_dir": r.workDir,
			"backend":     backendName,
		})
	}

	startTime := time.Now()
	err := cmd.Run()
	obs.DurationMs = time.Since(startTime).Milliseconds()

	// Capture raw output.
	rawStdout := stdoutBuf.String()
	rawStderr := stderrBuf.String()

	// Build truncated summaries for the Observation.
	obs.StdoutSummary = protocol.TruncateOutput(rawStdout)
	obs.StderrSummary = protocol.TruncateOutput(rawStderr)

	// Classify the result.
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			obs.Status = protocol.ObsStatusTimeout
			obs.Error = fmt.Sprintf("command timed out after %s", timeout)
			obs.ExitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			obs.Status = protocol.ObsStatusFailed
			obs.ExitCode = exitErr.ExitCode()
			obs.Error = fmt.Sprintf("command exited with code %d", obs.ExitCode)
		} else {
			obs.Status = protocol.ObsStatusFailed
			obs.ExitCode = -1
			obs.Error = fmt.Sprintf("failed to execute command: %s", err.Error())
		}
	} else {
		obs.Status = protocol.ObsStatusCompleted
		obs.ExitCode = 0
	}

	// Emit process.finished trace event and persist output.
	if r.logger != nil {
		r.logger.LogEvent(trace.EventTypeProcessFinished, "Process finished", map[string]interface{}{
			"exit_code":   obs.ExitCode,
			"duration_ms": obs.DurationMs,
			"status":      string(obs.Status),
			"backend":     backendName,
		})

		r.logger.WriteStdout(rawStdout)
		r.logger.WriteStderr(rawStderr)
	}

	return obs, nil
}

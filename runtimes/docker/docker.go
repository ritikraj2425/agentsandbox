// Package docker implements the Docker container runtime backend.
//
// This backend executes agent commands inside ephemeral Docker containers,
// providing filesystem and process isolation. The host workspace is bind-mounted
// into the container at /workspace, allowing the agent to read and modify
// project files while remaining sandboxed from the rest of the host system.
//
// Container lifecycle:
//  1. `docker create` — configure the container (image, mounts, command).
//  2. `docker start -a` — attach and run, capturing stdout/stderr.
//  3. `docker rm -f` — destroy the container regardless of outcome.
//
// Requirements:
//   - Docker daemon must be running and accessible via the `docker` CLI.
//   - The user must have permission to run `docker` commands (e.g., docker group).
package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// backendName is the canonical identifier for this runtime.
const backendName = "docker"

// DefaultImage is used when no --image flag is provided.
const DefaultImage = "alpine:latest"

// DefaultTimeout specifies the maximum duration a containerized command can
// execute before being forcefully terminated.
const DefaultTimeout = 120 * time.Second

func init() {
	// Self-register with the runtime registry so the CLI can discover
	// this backend via the --backend flag.
	runtime.Register(backendName, func(workDir string) (runtime.Runtime, error) {
		return New(workDir, DefaultImage, nil)
	})
}

// Runtime executes actions inside ephemeral Docker containers.
// It satisfies the runtime.Runtime interface.
type Runtime struct {
	// workDir is the host directory to bind-mount into the container.
	workDir string

	// image is the Docker image to use for the container (e.g., "golang:1.22").
	image string

	// timeout overrides the default command timeout. Zero means use DefaultTimeout.
	timeout time.Duration

	// logger is the optional trace logger for persisting execution events.
	logger *trace.RunLogger
}

// New creates a new Docker Runtime. It verifies that the Docker daemon is
// accessible before returning. Returns an error if Docker is unavailable.
func New(workDir, image string, logger *trace.RunLogger) (*Runtime, error) {
	// Verify Docker is available by running `docker info`.
	checkCmd := exec.Command("docker", "info")
	if err := checkCmd.Run(); err != nil {
		return nil, fmt.Errorf("docker is not available: %w (is Docker Desktop running?)", err)
	}

	if image == "" {
		image = DefaultImage
	}

	return &Runtime{
		workDir: workDir,
		image:   image,
		logger:  logger,
	}, nil
}

// SetTimeout overrides the default container execution timeout.
func (r *Runtime) SetTimeout(d time.Duration) {
	r.timeout = d
}

// Name returns the canonical backend identifier.
func (r *Runtime) Name() string {
	return backendName
}

// Run executes a shell command inside an ephemeral Docker container and
// returns the resulting Observation.
//
// The container is configured with:
//   - The specified image (e.g., golang:1.22, node:20, python:3.12).
//   - A bind mount of the host workspace to /workspace.
//   - /workspace as the working directory.
//   - The command executed via /bin/sh -c for shell compatibility.
//   - Automatic removal after execution (--rm equivalent via manual cleanup).
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

	// Build the `docker run` command.
	//
	// Flags:
	//   --rm           Automatically remove the container when it exits.
	//   -v             Bind-mount the host workspace into the container.
	//   -w /workspace  Set the working directory inside the container.
	//   /bin/sh -c     Execute the command through a shell for pipe/glob support.
	dockerArgs := r.dockerArgs(obs.Command)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	// Capture stdout and stderr into in-memory buffers.
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	// Emit process.started trace event.
	if r.logger != nil {
		r.logger.LogEvent(trace.EventTypeProcessStarted, "Docker container started", map[string]interface{}{
			"command":     obs.Command,
			"image":       r.image,
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
			obs.Error = fmt.Sprintf("container timed out after %s", timeout)
			obs.ExitCode = -1
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			obs.Status = protocol.ObsStatusFailed
			obs.ExitCode = exitErr.ExitCode()
			obs.Error = fmt.Sprintf("container exited with code %d", obs.ExitCode)
		} else {
			obs.Status = protocol.ObsStatusFailed
			obs.ExitCode = -1
			// Provide a helpful error if Docker is unavailable.
			errMsg := err.Error()
			if strings.Contains(errMsg, "Cannot connect to the Docker daemon") ||
				strings.Contains(errMsg, "docker: not found") {
				obs.Error = fmt.Sprintf("docker is not available: %s", errMsg)
			} else {
				obs.Error = fmt.Sprintf("failed to execute container: %s", errMsg)
			}
		}
	} else {
		obs.Status = protocol.ObsStatusCompleted
		obs.ExitCode = 0
	}

	// Emit process.finished trace event and persist output.
	if r.logger != nil {
		r.logger.LogEvent(trace.EventTypeProcessFinished, "Docker container finished", map[string]interface{}{
			"exit_code":   obs.ExitCode,
			"duration_ms": obs.DurationMs,
			"status":      string(obs.Status),
			"image":       r.image,
			"backend":     backendName,
		})

		r.logger.WriteStdout(rawStdout)
		r.logger.WriteStderr(rawStderr)
	}

	return obs, nil
}

func (r *Runtime) dockerArgs(command string) []string {
	return []string{
		"run",
		"--rm",
		"-v", fmt.Sprintf("%s:/workspace", r.workDir),
		"-w", "/workspace",
		r.image,
		"/bin/sh", "-c", command,
	}
}

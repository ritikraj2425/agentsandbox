// Package gvisor implements the gVisor container runtime backend.
//
// This backend executes agent commands inside Docker containers using the
// `runsc` runtime, which intercepts application system calls and acts as the
// guest kernel. This provides a strong process isolation boundary while
// retaining the lightweight nature of containers.
//
// Container lifecycle:
//  1. `docker create --runtime=runsc` — configure the container (image, mounts, cmd).
//  2. `docker start -a` — attach and run, capturing stdout/stderr.
//  3. `docker rm -f` — destroy the container regardless of outcome.
package gvisor

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

const backendName = "gvisor"
const DefaultImage = "alpine:latest"
const DefaultTimeout = 120 * time.Second

func init() {
	runtime.Register(backendName, func(workDir string) (runtime.Runtime, error) {
		return New(workDir, DefaultImage, "", "", nil)
	})
}

// Runtime executes actions inside ephemeral gVisor containers.
// It satisfies the runtime.Runtime interface.
type Runtime struct {
	workDir string
	image   string
	cpus    string
	memory  string
	timeout time.Duration
	logger  *trace.RunLogger
}

// New creates a new gVisor Runtime. It verifies that Docker is available and
// that the runsc runtime is configured before returning.
func New(workDir, image, cpus, memory string, logger *trace.RunLogger) (*Runtime, error) {
	// Verify Docker is available and check for runsc runtime.
	checkCmd := exec.Command("docker", "info", "--format", "{{.Runtimes}}")
	out, err := checkCmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker is not available: %w (is Docker Desktop running?)", err)
	}

	if !strings.Contains(string(out), "runsc") {
		return nil, fmt.Errorf("gvisor (runsc) runtime is not configured in Docker. Please install gVisor and configure dockerd to use it")
	}

	if image == "" {
		image = DefaultImage
	}

	return &Runtime{
		workDir: workDir,
		image:   image,
		cpus:    cpus,
		memory:  memory,
		logger:  logger,
	}, nil
}

func (r *Runtime) SetTimeout(d time.Duration) {
	r.timeout = d
}

func (r *Runtime) Name() string {
	return backendName
}

func (r *Runtime) Run(ctx context.Context, action protocol.Action) (protocol.Observation, error) {
	obs := protocol.NewObservation(action.ID)
	obs.Command = action.Command()
	obs.Backend = backendName

	if obs.Command == "" {
		obs.Status = protocol.ObsStatusFailed
		obs.Error = "no command specified in action parameters"
		return obs, fmt.Errorf("no command specified in action parameters")
	}

	timeout := r.timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the `docker run` command with `--runtime=runsc`
	dockerArgs := []string{
		"run",
		"--rm",
		"--runtime=runsc",
		"-v", fmt.Sprintf("%s:/workspace", r.workDir),
		"-w", "/workspace",
	}

	// Enforce strict resource limits if specified
	if r.cpus != "" {
		dockerArgs = append(dockerArgs, "--cpus", r.cpus)
	}
	if r.memory != "" {
		dockerArgs = append(dockerArgs, "--memory", r.memory)
	}

	dockerArgs = append(dockerArgs, r.image, "/bin/sh", "-c", obs.Command)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	if r.logger != nil {
		r.logger.LogEvent(trace.EventTypeProcessStarted, "gVisor container started", map[string]interface{}{
			"command":     obs.Command,
			"image":       r.image,
			"cpus":        r.cpus,
			"memory":      r.memory,
			"working_dir": r.workDir,
			"backend":     backendName,
		})
	}

	startTime := time.Now()
	err := cmd.Run()
	obs.DurationMs = time.Since(startTime).Milliseconds()

	rawStdout := stdoutBuf.String()
	rawStderr := stderrBuf.String()

	obs.StdoutSummary = protocol.TruncateOutput(rawStdout)
	obs.StderrSummary = protocol.TruncateOutput(rawStderr)

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
			obs.Error = fmt.Sprintf("failed to execute container: %s", err.Error())
		}
	} else {
		obs.Status = protocol.ObsStatusCompleted
		obs.ExitCode = 0
	}

	if r.logger != nil {
		r.logger.LogEvent(trace.EventTypeProcessFinished, "gVisor container finished", map[string]interface{}{
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

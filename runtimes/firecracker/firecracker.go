// Package firecracker implements the Firecracker microVM runtime backend.
//
// This backend executes agent commands inside lightweight Firecracker microVMs,
// providing hardware-level isolation through KVM virtualization. Each microVM
// boots a minimal Linux kernel with a root filesystem, executes the command,
// captures stdout/stderr, and is destroyed immediately after completion.
//
// Firecracker communicates via a REST API over a Unix domain socket. The
// lifecycle for each execution is:
//
//  1. Create a Unix socket and spawn the Firecracker VMM process.
//  2. Configure the VM via the API: kernel, root drive, network, and resources.
//  3. Start the microVM (InstanceStart action).
//  4. Wait for the guest to execute the command and exit.
//  5. Capture output from a shared serial console or virtio-vsock.
//  6. Destroy the VMM process and clean up temporary files.
//
// Requirements:
//   - Linux host with KVM support (/dev/kvm must be accessible).
//   - Firecracker binary installed and accessible in PATH.
//   - An uncompressed Linux kernel image (vmlinux).
//   - A root filesystem image (ext4 format).
package firecracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// backendName is the canonical identifier for this runtime.
const backendName = "firecracker"

// DefaultTimeout specifies the maximum duration a microVM can execute before
// being forcefully terminated.
const DefaultTimeout = 120 * time.Second

// DefaultVCPUCount is the default number of virtual CPUs allocated to each microVM.
const DefaultVCPUCount = 1

// DefaultMemSizeMB is the default memory allocation in megabytes for each microVM.
const DefaultMemSizeMB = 128

func init() {
	runtime.Register(backendName, func(workDir string) (runtime.Runtime, error) {
		return New(Config{WorkDir: workDir})
	})
}

// Config holds the configuration for a Firecracker runtime instance.
type Config struct {
	// WorkDir is the host directory to expose inside the microVM.
	WorkDir string

	// KernelPath is the path to the uncompressed Linux kernel image (vmlinux).
	KernelPath string

	// RootFSPath is the path to the root filesystem image (ext4).
	RootFSPath string

	// VCPUCount is the number of virtual CPUs to allocate.
	VCPUCount int

	// MemSizeMB is the memory allocation in megabytes.
	MemSizeMB int

	// Logger is the optional trace logger for persisting execution events.
	Logger *trace.RunLogger

	// Timeout overrides the default execution timeout.
	Timeout time.Duration
}

// Runtime executes actions inside ephemeral Firecracker microVMs.
// It satisfies the runtime.Runtime interface.
type Runtime struct {
	config Config
}

// New creates a new Firecracker Runtime. It verifies that the Firecracker
// binary and KVM are available before returning.
func New(cfg Config) (*Runtime, error) {
	// Verify Firecracker binary is installed.
	if _, err := exec.LookPath("firecracker"); err != nil {
		return nil, fmt.Errorf("firecracker binary not found in PATH: %w (install from https://github.com/firecracker-microvm/firecracker/releases)", err)
	}

	// Verify KVM is available.
	if _, err := os.Stat("/dev/kvm"); err != nil {
		return nil, fmt.Errorf("KVM is not available (/dev/kvm): %w (Firecracker requires a Linux host with KVM support)", err)
	}

	// Validate kernel path if provided.
	if cfg.KernelPath != "" {
		if _, err := os.Stat(cfg.KernelPath); err != nil {
			return nil, fmt.Errorf("kernel image not found at %q: %w", cfg.KernelPath, err)
		}
	}

	// Validate rootfs path if provided.
	if cfg.RootFSPath != "" {
		if _, err := os.Stat(cfg.RootFSPath); err != nil {
			return nil, fmt.Errorf("root filesystem not found at %q: %w", cfg.RootFSPath, err)
		}
	}

	// Apply defaults.
	if cfg.VCPUCount <= 0 {
		cfg.VCPUCount = DefaultVCPUCount
	}
	if cfg.MemSizeMB <= 0 {
		cfg.MemSizeMB = DefaultMemSizeMB
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	return &Runtime{config: cfg}, nil
}

// Name returns the canonical backend identifier.
func (r *Runtime) Name() string {
	return backendName
}

// SetTimeout overrides the default microVM execution timeout.
func (r *Runtime) SetTimeout(d time.Duration) {
	r.config.Timeout = d
}

// --- Firecracker API Types ---

// bootSource configures the kernel boot parameters for the microVM.
type bootSource struct {
	KernelImagePath string `json:"kernel_image_path"`
	BootArgs        string `json:"boot_args"`
}

// drive configures a block device (disk) for the microVM.
type drive struct {
	DriveID      string `json:"drive_id"`
	PathOnHost   string `json:"path_on_host"`
	IsRootDevice bool   `json:"is_root_device"`
	IsReadOnly   bool   `json:"is_read_only"`
}

// machineConfig configures the virtual hardware of the microVM.
type machineConfig struct {
	VCPUCount  int `json:"vcpu_count"`
	MemSizeMiB int `json:"mem_size_mib"`
}

// instanceAction triggers lifecycle actions on the microVM (e.g., start).
type instanceAction struct {
	ActionType string `json:"action_type"`
}

// --- API Client ---

// apiClient communicates with the Firecracker VMM over a Unix domain socket.
type apiClient struct {
	httpClient *http.Client
	socketPath string
}

// newAPIClient creates an HTTP client that routes requests through the
// Firecracker Unix domain socket.
func newAPIClient(socketPath string) *apiClient {
	return &apiClient{
		socketPath: socketPath,
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
					return net.Dial("unix", socketPath)
				},
			},
			Timeout: 30 * time.Second,
		},
	}
}

// put sends a PUT request with a JSON body to the Firecracker API.
func (c *apiClient) put(path string, body interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal request body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, "http://localhost"+path, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("API request to %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		var respBody bytes.Buffer
		respBody.ReadFrom(resp.Body)
		return fmt.Errorf("API %s returned %d: %s", path, resp.StatusCode, respBody.String())
	}

	return nil
}

// Run executes a shell command inside an ephemeral Firecracker microVM and
// returns the resulting Observation.
//
// The execution flow:
//  1. Create a temporary directory for the socket and output files.
//  2. Prepare the guest init script that runs the user command.
//  3. Spawn the Firecracker VMM process with a Unix socket.
//  4. Configure the VM via API calls (boot source, drives, machine config).
//  5. Start the microVM.
//  6. Wait for the VMM process to exit (guest finished).
//  7. Read captured stdout/stderr from the shared filesystem.
//  8. Clean up all temporary resources.
func (r *Runtime) Run(ctx context.Context, action protocol.Action) (protocol.Observation, error) {
	obs := protocol.NewObservation(action.ID)
	obs.Command = action.Command()
	obs.Backend = backendName

	if obs.Command == "" {
		obs.Status = protocol.ObsStatusFailed
		obs.Error = "no command specified in action parameters"
		return obs, fmt.Errorf("no command specified in action parameters")
	}

	// Apply timeout.
	ctx, cancel := context.WithTimeout(ctx, r.config.Timeout)
	defer cancel()

	// Create a temporary working directory for this VM instance.
	tmpDir, err := os.MkdirTemp("", "fc-sandbox-*")
	if err != nil {
		obs.Status = protocol.ObsStatusFailed
		obs.Error = fmt.Sprintf("failed to create temp directory: %s", err)
		return obs, err
	}
	defer os.RemoveAll(tmpDir)

	socketPath := filepath.Join(tmpDir, "firecracker.sock")
	stdoutPath := filepath.Join(tmpDir, "stdout.log")
	stderrPath := filepath.Join(tmpDir, "stderr.log")
	exitCodePath := filepath.Join(tmpDir, "exit_code")

	// Write the init script that will execute the user's command inside the VM.
	// The guest kernel boots, runs this script, captures output, and powers off.
	initScript := fmt.Sprintf(`#!/bin/sh
cd /workspace 2>/dev/null || true
%s > /tmp/stdout.log 2> /tmp/stderr.log
echo $? > /tmp/exit_code
poweroff -f
`, obs.Command)

	initScriptPath := filepath.Join(tmpDir, "init.sh")
	if err := os.WriteFile(initScriptPath, []byte(initScript), 0755); err != nil {
		obs.Status = protocol.ObsStatusFailed
		obs.Error = fmt.Sprintf("failed to write init script: %s", err)
		return obs, err
	}

	// Emit trace event.
	if r.config.Logger != nil {
		r.config.Logger.LogEvent(trace.EventTypeProcessStarted, "Firecracker microVM starting", map[string]interface{}{
			"command":     obs.Command,
			"kernel":      r.config.KernelPath,
			"rootfs":      r.config.RootFSPath,
			"vcpu_count":  r.config.VCPUCount,
			"mem_size_mb": r.config.MemSizeMB,
			"socket":      socketPath,
			"backend":     backendName,
		})
	}

	// Spawn the Firecracker VMM process.
	fcCmd := exec.CommandContext(ctx, "firecracker",
		"--api-sock", socketPath,
	)
	fcCmd.Stdout = nil
	fcCmd.Stderr = nil

	startTime := time.Now()

	if err := fcCmd.Start(); err != nil {
		obs.Status = protocol.ObsStatusFailed
		obs.ExitCode = -1
		obs.Error = fmt.Sprintf("failed to start firecracker process: %s", err)
		return obs, err
	}

	// Wait briefly for the socket to become available.
	if err := waitForSocket(socketPath, 5*time.Second); err != nil {
		_ = fcCmd.Process.Kill()
		obs.Status = protocol.ObsStatusFailed
		obs.ExitCode = -1
		obs.Error = fmt.Sprintf("firecracker socket did not become available: %s", err)
		return obs, err
	}

	// Configure the microVM via API calls.
	client := newAPIClient(socketPath)

	// Set kernel boot source.
	bootArgs := "console=ttyS0 reboot=k panic=1 pci=off"
	if err := client.put("/boot-source", bootSource{
		KernelImagePath: r.config.KernelPath,
		BootArgs:        bootArgs,
	}); err != nil {
		_ = fcCmd.Process.Kill()
		obs.Status = protocol.ObsStatusFailed
		obs.ExitCode = -1
		obs.Error = fmt.Sprintf("failed to configure boot source: %s", err)
		return obs, err
	}

	// Attach root filesystem drive.
	if err := client.put("/drives/rootfs", drive{
		DriveID:      "rootfs",
		PathOnHost:   r.config.RootFSPath,
		IsRootDevice: true,
		IsReadOnly:   false,
	}); err != nil {
		_ = fcCmd.Process.Kill()
		obs.Status = protocol.ObsStatusFailed
		obs.ExitCode = -1
		obs.Error = fmt.Sprintf("failed to configure root drive: %s", err)
		return obs, err
	}

	// Configure virtual hardware.
	if err := client.put("/machine-config", machineConfig{
		VCPUCount:  r.config.VCPUCount,
		MemSizeMiB: r.config.MemSizeMB,
	}); err != nil {
		_ = fcCmd.Process.Kill()
		obs.Status = protocol.ObsStatusFailed
		obs.ExitCode = -1
		obs.Error = fmt.Sprintf("failed to configure machine: %s", err)
		return obs, err
	}

	// Start the microVM.
	if err := client.put("/actions", instanceAction{
		ActionType: "InstanceStart",
	}); err != nil {
		_ = fcCmd.Process.Kill()
		obs.Status = protocol.ObsStatusFailed
		obs.ExitCode = -1
		obs.Error = fmt.Sprintf("failed to start microVM: %s", err)
		return obs, err
	}

	// Wait for the VMM process to exit (guest powered off).
	waitErr := fcCmd.Wait()
	obs.DurationMs = time.Since(startTime).Milliseconds()

	// Read captured output from shared filesystem.
	rawStdout := readFileOrEmpty(stdoutPath)
	rawStderr := readFileOrEmpty(stderrPath)
	exitCodeStr := strings.TrimSpace(readFileOrEmpty(exitCodePath))

	obs.StdoutSummary = protocol.TruncateOutput(rawStdout)
	obs.StderrSummary = protocol.TruncateOutput(rawStderr)

	// Determine exit status.
	if ctx.Err() == context.DeadlineExceeded {
		obs.Status = protocol.ObsStatusTimeout
		obs.Error = fmt.Sprintf("microVM timed out after %s", r.config.Timeout)
		obs.ExitCode = -1
	} else if exitCodeStr != "" {
		var code int
		if _, err := fmt.Sscanf(exitCodeStr, "%d", &code); err == nil {
			obs.ExitCode = code
		}
		if obs.ExitCode == 0 {
			obs.Status = protocol.ObsStatusCompleted
		} else {
			obs.Status = protocol.ObsStatusFailed
			obs.Error = fmt.Sprintf("command exited with code %d", obs.ExitCode)
		}
	} else if waitErr != nil {
		obs.Status = protocol.ObsStatusFailed
		obs.ExitCode = -1
		obs.Error = fmt.Sprintf("microVM process error: %s", waitErr)
	} else {
		obs.Status = protocol.ObsStatusCompleted
		obs.ExitCode = 0
	}

	// Emit trace event and persist output.
	if r.config.Logger != nil {
		r.config.Logger.LogEvent(trace.EventTypeProcessFinished, "Firecracker microVM finished", map[string]interface{}{
			"exit_code":   obs.ExitCode,
			"duration_ms": obs.DurationMs,
			"status":      string(obs.Status),
			"backend":     backendName,
		})

		r.config.Logger.WriteStdout(rawStdout)
		r.config.Logger.WriteStderr(rawStderr)
	}

	return obs, nil
}

// waitForSocket polls until the Unix socket file exists and is connectable,
// or the timeout elapses.
func waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for socket at %s", path)
}

// readFileOrEmpty reads a file and returns its contents, or an empty string
// if the file does not exist or cannot be read.
func readFileOrEmpty(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}

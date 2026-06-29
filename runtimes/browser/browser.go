// Package browser implements the browser tab sandbox runtime backend.
//
// This runtime orchestrates a headless Chromium instance inside a Docker
// container with a virtual framebuffer (Xvfb). It connects to Chrome's
// DevTools Protocol (CDP) and dispatches browser actions: navigation,
// clicking, typing, and screenshot capture.
//
// Architecture:
//
//	┌──────────────────────────────────────────────────┐
//	│         Docker Container                          │
//	│  ┌──────────┐   ┌────────────────────────────┐   │
//	│  │  Xvfb    │──▸│  Chromium (headless)        │   │
//	│  │ :99      │   │  --remote-debugging-port    │   │
//	│  └──────────┘   │  =9222                      │   │
//	│                 └──────────┬───────────────────┘   │
//	│                            │ CDP WebSocket         │
//	└────────────────────────────┼───────────────────────┘
//	                             │ Port mapped to host
//	                             ▼
//	┌──────────────────────────────────────────────────┐
//	│         CDPClient (perception package)            │
//	│   Navigate / Click / Type / Screenshot            │
//	└──────────────────────────────────────────────────┘
//
// The container runs the official Playwright Docker image which includes
// Chromium, fonts, and all required system libraries pre-installed.
package browser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/perception"
	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

const backendName = "browser"

// DefaultCDPPort is the default Chrome DevTools Protocol port inside the container.
const DefaultCDPPort = 9222

// DefaultTimeout is the maximum time for a single browser action.
const DefaultTimeout = 30 * time.Second

func init() {
	runtime.Register(backendName, func(workDir string) (runtime.Runtime, error) {
		return New(Config{WorkDir: workDir})
	})
}

// Config holds configuration for the browser runtime.
type Config struct {
	// WorkDir is the host directory to mount into the container.
	WorkDir string

	// Image is the Docker image containing Chromium. Defaults to the
	// official Playwright image.
	Image string

	// CDPPort overrides the default CDP debugging port.
	CDPPort int

	// Logger is the optional trace logger.
	Logger *trace.RunLogger
}

// Runtime manages a headless Chromium container and dispatches browser actions.
type Runtime struct {
	config      Config
	containerID string
	cdpClient   *perception.CDPClient
	hostPort    string
	vncHostPort string
}

// New creates a new browser Runtime and validates that Docker is available.
func New(cfg Config) (*Runtime, error) {
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker is required for the browser backend: %w", err)
	}

	if cfg.Image == "" {
		cfg.Image = "mcr.microsoft.com/playwright:v1.48.0-noble"
	}
	if cfg.CDPPort == 0 {
		cfg.CDPPort = DefaultCDPPort
	}

	return &Runtime{config: cfg}, nil
}

// Name returns the backend identifier.
func (r *Runtime) Name() string {
	return backendName
}

// Start launches the Chromium container and connects the CDP client.
func (r *Runtime) Start(ctx context.Context) error {
	// Launch a Docker container with Chromium in headless mode.
	// The container runs:
	//   1. Xvfb on display :99 (virtual framebuffer)
	//   2. Chromium with --remote-debugging-port=9222
	//
	// Port 9222 is mapped to a random host port so multiple browser
	// sessions can run concurrently without conflicts.
	args := []string{
		"run", "-d", "--rm",
		"-p", fmt.Sprintf("0:%d", r.config.CDPPort),
		"-p", "0:6080",
		"--shm-size=256m",
		r.config.Image,
		"/bin/sh", "-c",
		fmt.Sprintf(
			"apt-get update && apt-get install -y --no-install-recommends x11vnc python3-websockify && rm -rf /var/lib/apt/lists/* && "+
				"node -e \"const net = require('net'); net.createServer(s => { s.on('error', () => {}); const c = net.createConnection({port: %d, host: '127.0.0.1'}); c.on('error', () => s.destroy()); s.pipe(c).pipe(s); }).listen(%d, '0.0.0.0');\" & "+
				"Xvfb :99 -screen 0 1280x720x24 & "+
				"export DISPLAY=:99 && "+
				"sleep 1 && "+
				"x11vnc -display :99 -nopw -listen 0.0.0.0 -rfbport 5900 -shared -forever -q & "+
				"websockify 6080 localhost:5900 --web /usr/share/novnc & "+
				"/ms-playwright/chromium-*/chrome-linux/chrome --headless --no-sandbox --disable-gpu "+
				"--remote-debugging-port=%d "+
				"--disable-dev-shm-usage "+
				"about:blank",
			r.config.CDPPort+1, r.config.CDPPort, r.config.CDPPort+1),
	}

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start browser container: %s (stderr: %s)", err, stderr.String())
	}

	r.containerID = strings.TrimSpace(stdout.String())

	// Discover the mapped host port.
	hostPort, err := r.getHostPort(ctx)
	if err != nil {
		logs, _ := exec.Command("docker", "logs", r.containerID).CombinedOutput()
		r.cleanup()
		return fmt.Errorf("failed to discover CDP port mapping: %w (container logs: %s)", err, string(logs))
	}
	r.hostPort = hostPort

	// Discover the mapped VNC/websockify host port (container port 6080).
	vncPort, err := r.getHostPortForContainer(ctx, 6080)
	if err != nil {
		// VNC is optional; log warning but do not fail.
		log.Printf("Warning: failed to discover VNC port mapping: %v", err)
	} else {
		r.vncHostPort = vncPort
	}

	// Wait for CDP to be ready, then connect.
	wsURL, err := r.waitForCDP(ctx, 30*time.Second)
	if err != nil {
		logs, _ := exec.Command("docker", "logs", r.containerID).CombinedOutput()
		r.cleanup()
		return fmt.Errorf("CDP endpoint did not become ready: %w (container logs: %s)", err, string(logs))
	}

	cdp, err := perception.NewCDPClient(wsURL, DefaultTimeout)
	if err != nil {
		logs, _ := exec.Command("docker", "logs", r.containerID).CombinedOutput()
		r.cleanup()
		return fmt.Errorf("failed to connect CDP client: %w (container logs: %s)", err, string(logs))
	}
	r.cdpClient = cdp

	if r.config.Logger != nil {
		r.config.Logger.LogEvent(trace.EventTypeProcessStarted, "Browser container started", map[string]interface{}{
			"container_id": r.containerID,
			"cdp_port":     r.hostPort,
			"image":        r.config.Image,
			"backend":      backendName,
		})
	}

	return nil
}

// Stop tears down the browser container and disconnects the CDP client.
func (r *Runtime) Stop() {
	if r.cdpClient != nil {
		r.cdpClient.Close()
		r.cdpClient = nil
	}
	r.cleanup()
}

// VNCPort returns the host port mapped to the container's websockify VNC port.
func (r *Runtime) VNCPort() string {
	return r.vncHostPort
}

// CDPClient returns the underlying CDP client for direct access.
func (r *Runtime) CDPClient() *perception.CDPClient {
	return r.cdpClient
}

// Run executes a browser action and returns the Observation.
// If the container is not running, it starts one automatically.
func (r *Runtime) Run(ctx context.Context, action protocol.Action) (protocol.Observation, error) {
	obs := protocol.NewObservation(action.ID)
	obs.Backend = backendName

	// Auto-start if needed.
	if r.cdpClient == nil {
		if err := r.Start(ctx); err != nil {
			obs.Status = protocol.ObsStatusFailed
			obs.Error = fmt.Sprintf("failed to start browser: %s", err)
			return obs, err
		}
	}

	startTime := time.Now()

	var err error
	switch action.Type {
	case protocol.ActionTypeBrowserGoto:
		err = r.handleGoto(action, &obs)
	case protocol.ActionTypeBrowserClick:
		err = r.handleClick(action, &obs)
	case protocol.ActionTypeBrowserType:
		err = r.handleType(action, &obs)
	case protocol.ActionTypeBrowserScreenshot:
		err = r.handleScreenshot(&obs)
	default:
		obs.Status = protocol.ObsStatusFailed
		obs.Error = fmt.Sprintf("unsupported browser action type: %s", action.Type)
		return obs, fmt.Errorf("unsupported action type: %s", action.Type)
	}

	obs.DurationMs = time.Since(startTime).Milliseconds()

	if err != nil {
		obs.Status = protocol.ObsStatusFailed
		obs.Error = err.Error()
		obs.ExitCode = 1
		return obs, err
	}

	obs.Status = protocol.ObsStatusCompleted
	obs.ExitCode = 0

	// Attach page metadata.
	if title, terr := r.cdpClient.GetTitle(); terr == nil {
		obs.PageTitle = title
	}
	if pageURL, uerr := r.cdpClient.GetURL(); uerr == nil {
		obs.PageURL = pageURL
	}

	if r.config.Logger != nil {
		r.config.Logger.LogEvent(trace.EventTypeProcessFinished, "Browser action completed", map[string]interface{}{
			"action_type": string(action.Type),
			"duration_ms": obs.DurationMs,
			"status":      string(obs.Status),
			"backend":     backendName,
		})
	}

	return obs, nil
}

// --- Action Handlers ---

func (r *Runtime) handleGoto(action protocol.Action, obs *protocol.Observation) error {
	url := action.URL()
	if url == "" {
		return fmt.Errorf("browser.goto requires a 'url' parameter")
	}
	obs.Command = fmt.Sprintf("browser.goto %s", url)
	return r.cdpClient.Navigate(url)
}

func (r *Runtime) handleClick(action protocol.Action, obs *protocol.Observation) error {
	// Try CSS selector first, fall back to coordinates.
	if selector := action.Selector(); selector != "" {
		obs.Command = fmt.Sprintf("browser.click(%s)", selector)
		return r.cdpClient.ClickSelector(selector)
	}

	x, y, ok := action.Coordinates()
	if !ok {
		return fmt.Errorf("browser.click requires 'selector' or 'x'/'y' parameters")
	}
	obs.Command = fmt.Sprintf("browser.click(%.0f, %.0f)", x, y)
	return r.cdpClient.Click(x, y)
}

func (r *Runtime) handleType(action protocol.Action, obs *protocol.Observation) error {
	text := action.Text()
	if text == "" {
		return fmt.Errorf("browser.type requires a 'text' parameter")
	}
	obs.Command = fmt.Sprintf("browser.type(%q)", text)
	return r.cdpClient.TypeText(text)
}

func (r *Runtime) handleScreenshot(obs *protocol.Observation) error {
	obs.Command = "browser.screenshot"
	base64Data, err := r.cdpClient.CaptureScreenshot()
	if err != nil {
		return err
	}
	obs.Screenshot = base64Data
	obs.StdoutSummary = fmt.Sprintf("[screenshot captured: %d bytes base64]", len(base64Data))
	return nil
}

// --- Internal Helpers ---

func (r *Runtime) getHostPort(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "port", r.containerID,
		fmt.Sprintf("%d/tcp", r.config.CDPPort))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker port lookup failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no port mapping found")
	}

	// Output line is like "0.0.0.0:55123" or "[::]:55123"
	mapping := strings.TrimSpace(lines[0])
	parts := strings.Split(mapping, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected port mapping format: %s", mapping)
	}
	return parts[len(parts)-1], nil
}

// getHostPortForContainer looks up the mapped host port for a specific container port.
func (r *Runtime) getHostPortForContainer(ctx context.Context, containerPort int) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "port", r.containerID,
		fmt.Sprintf("%d/tcp", containerPort))
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("docker port lookup failed for port %d: %w", containerPort, err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return "", fmt.Errorf("no port mapping found for port %d", containerPort)
	}

	mapping := strings.TrimSpace(lines[0])
	parts := strings.Split(mapping, ":")
	if len(parts) < 2 {
		return "", fmt.Errorf("unexpected port mapping format: %s", mapping)
	}
	return parts[len(parts)-1], nil
}

// waitForCDP polls the Chrome DevTools JSON endpoint until it responds,
// then returns the WebSocket URL for the first available page target.
func (r *Runtime) waitForCDP(ctx context.Context, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	httpClient := &http.Client{Timeout: 2 * time.Second}

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		url := fmt.Sprintf("http://localhost:%s/json", r.hostPort)
		resp, err := httpClient.Get(url)
		if err != nil {
			time.Sleep(300 * time.Millisecond)
			continue
		}

		var targets []struct {
			Type               string `json:"type"`
			WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&targets); err != nil {
			resp.Body.Close()
			time.Sleep(300 * time.Millisecond)
			continue
		}
		resp.Body.Close()

		// Find the first "page" target.
		for _, t := range targets {
			if t.Type == "page" && t.WebSocketDebuggerURL != "" {
				return t.WebSocketDebuggerURL, nil
			}
		}

		time.Sleep(300 * time.Millisecond)
	}

	return "", fmt.Errorf("CDP did not respond within %s", timeout)
}

// cleanup removes the Docker container.
func (r *Runtime) cleanup() {
	if r.containerID == "" {
		return
	}
	exec.Command("docker", "rm", "-f", r.containerID).Run()
	r.containerID = ""
}

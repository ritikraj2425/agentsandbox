// Package sdk provides the high-level Go SDK for integrating with AgentSandbox.
//
// This is the primary package that external developers import to interact with
// an AgentSandbox gateway. It wraps the low-level client package with ergonomic
// session management, automatic cleanup, and a fluent API.
//
// Quick Start:
//
//	sandbox := sdk.NewClient("http://localhost:8080", "secret_token")
//
//	session, err := sandbox.CreateSession(sdk.SessionConfig{
//	    Backend: "docker",
//	    Image:   "golang:1.26",
//	    MemoryLimit: "1gb",
//	})
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer session.Destroy()
//
//	obs, err := session.Run("go test ./...")
//	fmt.Println(obs.StdoutSummary)
package sdk

import (
	"fmt"
	"time"

	"github.com/ritikraj2425/agentsandbox/pkg/client"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// Client is the top-level SDK entry point. It manages communication with
// an AgentSandbox gateway and provides session lifecycle methods.
type Client struct {
	api *client.Client
}

// NewClient creates a new SDK client targeting the given gateway.
//
//	sandbox := sdk.NewClient("http://localhost:8080", "my_api_key")
func NewClient(baseURL string, token string) *Client {
	return &Client{
		api: client.New(baseURL, token),
	}
}

// SessionConfig specifies how a sandbox session should be created.
type SessionConfig struct {
	// Backend selects the execution runtime: "local", "docker", "gvisor", "firecracker".
	Backend string

	// Image is the container image to use (required for docker/gvisor backends).
	Image string

	// CPULimit restricts the CPU cores available (e.g., "1.5").
	CPULimit string

	// MemoryLimit restricts memory available (e.g., "512mb", "2gb").
	MemoryLimit string

	// TTL is how long the session stays alive before automatic cleanup.
	// Defaults to 1 hour if zero.
	TTL time.Duration

	// Policy selects a bundled policy by name (for example "coding-safe").
	Policy string

	// PolicyFile selects a policy by file path.
	PolicyFile string

	// Workspace configures optional session workspace initialization.
	Workspace protocol.WorkspaceInitRequest
}

// Session represents an active sandbox environment on the gateway.
// It provides methods to execute commands and manage the session lifecycle.
type Session struct {
	// ID is the unique session identifier assigned by the gateway.
	ID string

	// ExpiresAt is when this session will be automatically destroyed.
	ExpiresAt time.Time

	// Config is the original configuration used to create this session.
	Config SessionConfig

	// api is the underlying HTTP client.
	api *client.Client
}

// CreateSession instantiates a new virtual sandbox on the gateway.
// The returned Session should be destroyed when no longer needed:
//
//	session, _ := sandbox.CreateSession(config)
//	defer session.Destroy()
func (c *Client) CreateSession(cfg SessionConfig) (*Session, error) {
	if cfg.Backend == "" {
		cfg.Backend = "local"
	}
	if cfg.TTL == 0 {
		cfg.TTL = 1 * time.Hour
	}

	resp, err := c.api.CreateSession(client.CreateSessionRequest{
		Backend:    cfg.Backend,
		Image:      cfg.Image,
		CPUs:       cfg.CPULimit,
		Memory:     cfg.MemoryLimit,
		TTL:        cfg.TTL,
		Policy:     cfg.Policy,
		PolicyFile: cfg.PolicyFile,
		Workspace:  cfg.Workspace,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &Session{
		ID:        resp.SessionID,
		ExpiresAt: resp.ExpiresAt,
		Config:    cfg,
		api:       c.api,
	}, nil
}

// Run executes a shell command in this sandbox session and returns the Observation.
//
//	obs, err := session.Run("echo hello")
//	fmt.Println(obs.StdoutSummary) // "hello"
func (s *Session) Run(command string) (*protocol.Observation, error) {
	obs, err := s.api.RunAction(s.ID, client.RunActionRequest{
		Command: command,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run action: %w", err)
	}
	return obs, nil
}

// Execute runs an Action (from the protocol package) in this sandbox session.
// This is useful when constructing actions programmatically.
//
//	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
//	    "command": "go test ./...",
//	})
//	obs, err := session.Execute(action)
func (s *Session) Execute(action protocol.Action) (*protocol.Observation, error) {
	obs, err := s.api.RunAction(s.ID, client.RunActionRequest{
		Type:           action.Type,
		Parameters:     action.Parameters,
		ClientActionID: action.ID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to run action: %w", err)
	}
	return obs, nil
}

// Destroy tears down the sandbox session and releases all resources.
// Always call this when done (or use defer).
func (s *Session) Destroy() error {
	if err := s.api.DeleteSession(s.ID); err != nil {
		return fmt.Errorf("failed to destroy session %s: %w", s.ID, err)
	}
	return nil
}

// IsExpired returns true if the session's TTL has elapsed.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

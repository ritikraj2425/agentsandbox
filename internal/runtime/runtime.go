// Package runtime defines the standard execution interface for all sandbox backends.
//
// Every execution environment — local shell, Docker container, gVisor sandbox,
// or Firecracker microVM — implements the Runtime interface. This provides a
// single, uniform contract that the CLI, API gateway, and scheduler depend on,
// completely decoupling the orchestration layer from backend-specific concerns.
//
// The interface is intentionally minimal: Name() for identification and Run()
// for execution. Backend-specific configuration (timeouts, resource limits,
// network policies) belongs in the concrete implementation's constructor.
package runtime

import (
	"context"
	"fmt"

	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// Runtime is the universal execution interface that all sandbox backends
// must implement. The CLI and API layer interact exclusively through this
// interface, enabling seamless backend substitution.
type Runtime interface {
	// Name returns a human-readable identifier for this backend (e.g., "local",
	// "docker", "gvisor"). Used in Observations, traces, and CLI output.
	Name() string

	// Run executes the given action within the backend's execution environment
	// and returns the resulting Observation. The context controls cancellation
	// and timeout propagation.
	Run(ctx context.Context, action protocol.Action) (protocol.Observation, error)
}

// ErrUnknownBackend is returned when the CLI requests a backend that has not
// been registered with the runtime registry.
var ErrUnknownBackend = fmt.Errorf("unknown backend")

// Registry maps backend names to their constructor functions. This allows the
// CLI to resolve a --backend flag string into a concrete Runtime without
// importing every backend package directly.
var Registry = map[string]func(workDir string) (Runtime, error){}

// Register adds a backend constructor to the global registry. Each backend
// package calls this from its init() function to self-register.
func Register(name string, factory func(workDir string) (Runtime, error)) {
	Registry[name] = factory
}

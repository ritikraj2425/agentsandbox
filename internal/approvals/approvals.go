// Package approvals implements human-in-the-loop workflows for the sandbox.
//
// When the security policy flags a command as "require_approval", the runtime
// suspends execution and delegates to this package to prompt the developer.
// This is critical for commands that are potentially destructive or expensive
// (like `git push` or `npm install`) but necessary for agent productivity.
package approvals

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/ritikraj2425/agentsandbox/internal/color"
)

// Request defines the contextual information presented to the human approver.
type Request struct {
	// Command is the exact shell string the agent is trying to execute.
	Command string

	// Reason explains why this command was flagged (e.g., matched policy rule).
	Reason string
}

// Config dictates how the approval prompt behaves, supporting automated
// environments (CI/CD) and aggressive bypasses.
type Config struct {
	// AutoApprove silently permits the command without prompting (--yes flag).
	AutoApprove bool

	// NonInteractive silently denies the command without prompting (--non-interactive flag).
	NonInteractive bool

	// Reader defines where user input comes from (typically os.Stdin).
	Reader io.Reader

	// Writer defines where the prompt is displayed (typically os.Stdout).
	Writer io.Writer
}

// Ask evaluates an approval request against the configuration.
//
// If running in an interactive terminal, it presents a [y/N] prompt to the user.
// It returns true if the command is permitted to execute, and false if denied.
func Ask(req Request, cfg Config) bool {
	// Short-circuit evaluations for automated environments.
	if cfg.AutoApprove {
		return true
	}
	if cfg.NonInteractive {
		return false
	}

	// Render the interactive prompt.
	fmt.Fprintf(cfg.Writer, "\n%s %s\n", color.Yellow("Command requires approval:"), req.Command)
	fmt.Fprintf(cfg.Writer, "%s %s\n", color.Yellow("Reason:"), color.Dim(req.Reason))
	fmt.Fprintf(cfg.Writer, "%s [y/N] ", color.BoldCyan("Allow?"))

	// Await human input.
	reader := bufio.NewReader(cfg.Reader)
	input, err := reader.ReadString('\n')
	if err != nil {
		// If the input stream closes (EOF) or errors (Ctrl+C), default to secure denial.
		fmt.Fprintln(cfg.Writer)
		return false
	}

	// Normalize input. We strictly require explicit 'y' or 'yes'.
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

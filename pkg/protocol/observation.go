package protocol

import "time"

// ObservationStatus indicates how the action execution concluded.
// The agent reads this to decide what to do next.
type ObservationStatus string

const (
	// ObsStatusCompleted means the action ran successfully.
	// The agent can inspect stdout/stderr and proceed.
	ObsStatusCompleted ObservationStatus = "completed"

	// ObsStatusFailed means the action ran but produced an error
	// (e.g., a shell command exited with non-zero code).
	// The agent can inspect stderr to understand the failure.
	ObsStatusFailed ObservationStatus = "failed"

	// ObsStatusDenied means the policy engine blocked the action
	// before it could execute. Nothing was run.
	// The agent should try a different approach.
	ObsStatusDenied ObservationStatus = "denied"

	// ObsStatusTimeout means the action exceeded its time limit
	// and was forcefully killed.
	ObsStatusTimeout ObservationStatus = "timeout"

	// ObsStatusApprovalRequired means the action needs human approval.
	// The agent should wait or try something else.
	ObsStatusApprovalRequired ObservationStatus = "approval_required"
)

// Observation is the structured result returned after an Action is processed.
//
// This is the OUTPUT of the sandbox. After the action runs (or is denied),
// the sandbox builds an Observation and sends it back to the agent.
//
// Design principle from the manifesto:
//
//	"Do not send the whole world back to the model. Send only what changed."
//
// That's why we have StdoutSummary (truncated/summarized output) instead
// of always sending the full stdout. The full output is stored on disk
// and referenced by StdoutPath for later inspection.
type Observation struct {
	// ActionID links this observation back to the Action that triggered it.
	// This allows the trace system to correlate actions with their results.
	ActionID string `json:"action_id"`

	// Status tells the agent what happened: completed, failed, denied, etc.
	Status ObservationStatus `json:"status"`

	// ExitCode is the process exit code for shell commands.
	// 0 = success, non-zero = failure. -1 means the process never ran
	// (e.g., denied by policy or failed to start).
	ExitCode int `json:"exit_code"`

	// DurationMs is how long the action took to execute in milliseconds.
	// This helps agents and benchmarks measure action latency.
	DurationMs int64 `json:"duration_ms"`

	// StdoutSummary contains the last N lines of stdout.
	// We truncate because AI models have context limits — sending 100K lines
	// of test output wastes tokens and slows down the agent loop.
	// The full output is always available at StdoutPath.
	StdoutSummary string `json:"stdout_summary"`

	// StderrSummary contains the last N lines of stderr.
	// Same truncation logic as StdoutSummary.
	StderrSummary string `json:"stderr_summary"`

	// Error is a human-readable error message when something went wrong.
	// For denied actions: "denied by policy: ..."
	// For failed commands: the stderr summary or exec error.
	// Empty string when the action succeeded.
	Error string `json:"error,omitempty"`

	// Command is the original command string (for shell actions).
	// Stored here so the observation is self-contained — you can understand
	// what happened without looking up the original Action.
	Command string `json:"command,omitempty"`

	// FilesChanged lists paths of files that were created or modified.
	FilesChanged []string `json:"files_changed,omitempty"`

	// FilesDeleted lists paths of files that were deleted.
	FilesDeleted []string `json:"files_deleted,omitempty"`

	// Backend identifies which runtime backend produced this observation
	// (e.g., "local", "docker", "gvisor"). Enables multi-backend auditing.
	Backend string `json:"backend,omitempty"`

	// Screenshot holds a base64-encoded PNG screenshot captured by
	// browser actions. Only populated for browser.screenshot actions.
	Screenshot string `json:"screenshot,omitempty"`

	// PageTitle is the current browser page title after a browser action.
	PageTitle string `json:"page_title,omitempty"`

	// PageURL is the current browser URL after a browser action.
	PageURL string `json:"page_url,omitempty"`

	// CreatedAt is when this observation was generated.
	CreatedAt time.Time `json:"created_at"`
}

// NewObservation creates a base Observation linked to the given action ID.
// The caller fills in the remaining fields after execution.
func NewObservation(actionID string) Observation {
	return Observation{
		ActionID:  actionID,
		ExitCode:  -1, // -1 = "never ran" until proven otherwise
		CreatedAt: time.Now(),
	}
}

// MaxSummaryLines controls how many lines of stdout/stderr are included
// in the observation summary. 50 lines is enough for the agent to understand
// what happened without overwhelming its context window.
const MaxSummaryLines = 50

// TruncateOutput takes raw output and returns the last MaxSummaryLines lines.
// If the output is shorter than the limit, it is returned unchanged.
//
// Why truncate from the END (keep last lines)?
// For test output: the final lines contain PASS/FAIL and error messages.
// For build output: the final lines contain the error summary.
// For ls/cat: the last lines are least useful, but these outputs are usually short.
func TruncateOutput(output string) string {
	lines := splitLines(output)
	if len(lines) <= MaxSummaryLines {
		return output
	}
	// Keep only the last MaxSummaryLines lines.
	start := len(lines) - MaxSummaryLines
	result := ""
	for i := start; i < len(lines); i++ {
		if i > start {
			result += "\n"
		}
		result += lines[i]
	}
	return "[truncated to last 50 lines]\n" + result
}

// splitLines splits a string by newline characters.
// Handles both \n and \r\n line endings.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	var lines []string
	current := ""
	for _, ch := range s {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else if ch == '\r' {
			// Skip carriage returns (handles \r\n Windows line endings).
			continue
		} else {
			current += string(ch)
		}
	}
	// Include the last line if it doesn't end with a newline.
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

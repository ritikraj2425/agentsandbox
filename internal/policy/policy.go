// Package policy implements the security policy engine that gates every
// agent action before it reaches the execution runtime.
//
// The policy engine is the security boundary of AgentSandbox. It sits
// between the action parser and the runner:
//
//	CLI/API  →  Action  →  Policy Engine  →  Runner  →  Observation
//	                         ↑ you are here
//
// If the policy denies an action, the runner is never called and the
// agent receives an Observation with status "denied".
//
// Design principles:
//   - Deny always wins over allow (defense in depth).
//   - Default is deny (allowlist model, not blocklist).
//   - Prefix matching for commands ("go test" matches "go test ./...").
//   - Policies are loaded from simple YAML files.
//   - No external YAML library — we parse the simple format ourselves.
package policy

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ritikraj2425/agentsandbox/internal/actions"
)

// ─────────────────────────────────────────────────────────────────────────────
// Policy Decision
// ─────────────────────────────────────────────────────────────────────────────

// Decision represents the outcome of a policy check against a command.
// It carries both the verdict and enough context to produce a useful
// log entry or error message.
type Decision struct {
	// Allowed is true if the command is permitted to execute.
	Allowed bool

	// Effect is the policy category that matched: "allow", "deny",
	// "require_approval", or "default_deny".
	Effect string

	// MatchedRule is the specific prefix pattern that triggered the
	// decision (e.g., "rm -rf"). Empty when no rule matched and the
	// default deny kicked in.
	MatchedRule string

	// Reason is a human-readable explanation suitable for logs and
	// terminal output (e.g., "denied by rule: rm -rf").
	Reason string
}

// ─────────────────────────────────────────────────────────────────────────────
// Command Policy (the new YAML-based policy for Phase 3)
// ─────────────────────────────────────────────────────────────────────────────

// CommandPolicy holds the parsed command-level policy rules.
//
// The YAML format is intentionally simple so anyone can read and write
// policies without learning a complex DSL:
//
//	name: coding-safe
//	commands:
//	  allow:
//	    - go test
//	    - gofmt
//	    - echo
//	  deny:
//	    - rm -rf
//	    - sudo
//	  require_approval:
//	    - npm install
//
// Evaluation order:
//  1. Check deny list first  — if the command starts with a denied prefix, block it.
//  2. Check allow list       — if the command starts with an allowed prefix, permit it.
//  3. Default deny           — anything not explicitly allowed is blocked.
//
// This "deny > allow > default deny" ordering is the most secure model.
// It means you can never accidentally allow a dangerous command by putting
// it in the allow list — deny always wins.
type CommandPolicy struct {
	// Name identifies this policy (e.g., "coding-safe", "research").
	Name string

	// Description explains what this policy is designed for.
	Description string

	// Allow is the list of command prefixes that are permitted.
	// A command matches if it starts with any prefix in this list.
	// Example: "go test" allows "go test ./...", "go test -v", etc.
	Allow []string

	// Deny is the list of command prefixes that are always blocked.
	// Deny rules are checked BEFORE allow rules — they always win.
	// Example: "rm -rf" blocks "rm -rf /", "rm -rf .", etc.
	Deny []string

	// RequireApproval is the list of command prefixes that need human
	// confirmation before executing. For Phase 3 we treat these as
	// denied with a special message; Phase 5 will add interactive approval.
	RequireApproval []string
}

// CheckCommand evaluates a shell command against this policy and returns
// a Decision describing what should happen.
//
// The matching algorithm uses prefix comparison:
//   - The command "go test ./..." starts with the prefix "go test"  → match
//   - The command "gofmt main.go" starts with the prefix "gofmt"   → match
//   - The command "rm -rf /"      starts with the prefix "rm -rf"  → match
//
// Why prefix matching instead of exact match or regex?
//   - Exact match would require listing every possible argument combination.
//   - Regex is powerful but easy to get wrong (security risk).
//   - Prefix matching is intuitive: "allow go test" means "allow any
//     command that starts with go test". Simple, predictable, safe.
//
// Prefix matching also handles word boundaries correctly:
//   - Prefix "echo" matches "echo hello" (prefix + space + args).
//   - Prefix "echo" matches "echo" exactly (prefix with no args).
//   - Prefix "echo" does NOT match "echoserver" (partial word match is rejected).
func (p *CommandPolicy) CheckCommand(command string) Decision {
	cmd := strings.TrimSpace(command)

	// Step 1: Check deny list first (deny always wins).
	for _, prefix := range p.Deny {
		if matchesPrefix(cmd, prefix) {
			return Decision{
				Allowed:     false,
				Effect:      "deny",
				MatchedRule: prefix,
				Reason:      fmt.Sprintf("blocked by deny rule: %q", prefix),
			}
		}
	}

	// Step 2: Check require_approval list.
	for _, prefix := range p.RequireApproval {
		if matchesPrefix(cmd, prefix) {
			return Decision{
				Allowed:     false,
				Effect:      "require_approval",
				MatchedRule: prefix,
				Reason:      fmt.Sprintf("requires approval: %q", prefix),
			}
		}
	}

	// Step 3: Check allow list.
	for _, prefix := range p.Allow {
		if matchesPrefix(cmd, prefix) {
			return Decision{
				Allowed:     true,
				Effect:      "allow",
				MatchedRule: prefix,
				Reason:      fmt.Sprintf("allowed by rule: %q", prefix),
			}
		}
	}

	// Step 4: Nothing matched — default deny.
	return Decision{
		Allowed:     false,
		Effect:      "default_deny",
		MatchedRule: "",
		Reason:      "denied by default: command not in allow list",
	}
}

// matchesPrefix checks if a command starts with a given policy prefix.
//
// The match is word-boundary-aware to prevent false positives:
//   - prefix "echo" matches "echo" (exact match)
//   - prefix "echo" matches "echo hello" (prefix + space)
//   - prefix "echo" does NOT match "echoserver" (partial word)
//
// This is critical for security: without word boundaries, a rule for
// "rm" would accidentally match "rmdir", which is a completely
// different and potentially safe command.
func matchesPrefix(command, prefix string) bool {
	if !strings.HasPrefix(command, prefix) {
		return false
	}
	// If the command is exactly the prefix, it's a match.
	if len(command) == len(prefix) {
		return true
	}
	// If the command is longer, the next character must be a space.
	// This ensures "echo" matches "echo hello" but not "echoserver".
	return command[len(prefix)] == ' '
}

// ─────────────────────────────────────────────────────────────────────────────
// YAML Loading (zero-dependency parser)
// ─────────────────────────────────────────────────────────────────────────────

// LoadCommandPolicyFromFile reads a YAML policy file and returns a
// parsed CommandPolicy.
//
// Why not use gopkg.in/yaml.v3?
//   - We committed to zero external dependencies during Phase 0.
//   - Our YAML format is intentionally simple: flat keys and string lists.
//   - A 60-line custom parser handles this perfectly and avoids pulling
//     in a full YAML library with recursive descent parsing, anchors,
//     aliases, and other features we'll never use.
//
// If the policy format grows more complex in the future (nested objects,
// conditional rules, schema validation), we can adopt a real YAML library.
func LoadCommandPolicyFromFile(path string) (*CommandPolicy, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open policy file %s: %w", path, err)
	}
	defer file.Close()

	policy := &CommandPolicy{}
	scanner := bufio.NewScanner(file)

	// currentSection tracks which list we're currently parsing.
	// It maps to the YAML structure:
	//   commands:
	//     allow:        ← currentSection = "allow"
	//       - go test
	//     deny:         ← currentSection = "deny"
	//       - rm -rf
	var currentSection string

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and comments.
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Detect top-level keys (no leading whitespace).
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Parse "key: value" pairs.
			if strings.HasPrefix(trimmed, "name:") {
				policy.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			} else if strings.HasPrefix(trimmed, "description:") {
				policy.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			} else if trimmed == "commands:" {
				// Entering the commands block — sections follow.
				currentSection = ""
			}
			continue
		}

		// Detect section headers under "commands:" (indented once).
		if (strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "\t")) &&
			!strings.HasPrefix(trimmed, "-") {
			// This is a section key like "allow:", "deny:", "require_approval:".
			sectionName := strings.TrimSuffix(trimmed, ":")
			switch sectionName {
			case "allow":
				currentSection = "allow"
			case "deny":
				currentSection = "deny"
			case "require_approval":
				currentSection = "require_approval"
			default:
				currentSection = ""
			}
			continue
		}

		// Parse list items (lines starting with "- ").
		if strings.HasPrefix(trimmed, "- ") {
			value := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			switch currentSection {
			case "allow":
				policy.Allow = append(policy.Allow, value)
			case "deny":
				policy.Deny = append(policy.Deny, value)
			case "require_approval":
				policy.RequireApproval = append(policy.RequireApproval, value)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading policy file %s: %w", path, err)
	}

	return policy, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Legacy Policy (backward compatibility with Phase 0/1)
// ─────────────────────────────────────────────────────────────────────────────
// The types below are preserved so that the existing Run() method in the
// runner package and its tests continue to compile and work unchanged.
// New code should use CommandPolicy and CheckCommand instead.

// Effect determines whether a legacy rule allows or denies an action.
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Rule defines a single legacy policy rule that matches actions by type.
type Rule struct {
	Action      string                 `json:"action" yaml:"action"`
	Effect      Effect                 `json:"effect" yaml:"effect"`
	Description string                 `json:"description,omitempty" yaml:"description,omitempty"`
	Conditions  map[string]interface{} `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// Policy is the legacy policy struct used by the old runner.Run() method.
type Policy struct {
	Name          string `json:"name" yaml:"name"`
	Version       string `json:"version" yaml:"version"`
	Description   string `json:"description,omitempty" yaml:"description,omitempty"`
	DefaultEffect Effect `json:"default_effect" yaml:"default_effect"`
	Rules         []Rule `json:"rules" yaml:"rules"`
}

// Evaluate checks an action against legacy policy rules.
// This method is preserved for backward compatibility with the runner's
// Run() method and its existing test suite.
func (p *Policy) Evaluate(action *actions.Action) (allowed bool, reason string) {
	actionType := string(action.Type)

	for _, rule := range p.Rules {
		if rule.Action == actionType || rule.Action == "*" {
			switch rule.Effect {
			case EffectAllow:
				return true, rule.Description
			case EffectDeny:
				if rule.Description != "" {
					return false, rule.Description
				}
				return false, "denied by rule"
			}
		}
	}

	switch p.DefaultEffect {
	case EffectAllow:
		return true, "allowed by default"
	default:
		return false, "denied by default (no matching rule)"
	}
}

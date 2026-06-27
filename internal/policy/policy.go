// Package policy defines the security policy engine that determines
// which agent actions are allowed or denied.
package policy

import (
	"github.com/ritikraj2425/agentsandbox/internal/actions"
)

// Effect determines whether a rule allows or denies an action.
type Effect string

const (
	EffectAllow Effect = "allow"
	EffectDeny  Effect = "deny"
)

// Rule defines a single policy rule that matches actions by type
// and applies an effect (allow/deny).
type Rule struct {
	// Action is the action type this rule matches (e.g., "shell", "file_write").
	Action string `json:"action" yaml:"action"`

	// Effect determines whether matching actions are allowed or denied.
	Effect Effect `json:"effect" yaml:"effect"`

	// Description is a human-readable explanation of the rule.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// Conditions holds optional constraints that further restrict the rule.
	Conditions map[string]interface{} `json:"conditions,omitempty" yaml:"conditions,omitempty"`
}

// Policy represents a complete security policy for evaluating agent actions.
type Policy struct {
	// Name identifies this policy.
	Name string `json:"name" yaml:"name"`

	// Version is the policy version string.
	Version string `json:"version" yaml:"version"`

	// Description explains the purpose of this policy.
	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	// DefaultEffect is the fallback effect when no rule matches.
	DefaultEffect Effect `json:"default_effect" yaml:"default_effect"`

	// Rules is the ordered list of policy rules. The first matching rule wins.
	Rules []Rule `json:"rules" yaml:"rules"`
}

// Evaluate checks the action against the policy rules and returns
// whether the action is allowed and a reason string.
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

	// No matching rule: use default effect.
	switch p.DefaultEffect {
	case EffectAllow:
		return true, "allowed by default"
	default:
		return false, "denied by default (no matching rule)"
	}
}

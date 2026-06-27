// Package policy provides tests for the Policy evaluation engine.
package policy

import (
	"testing"

	"github.com/ritikraj2425/agentsandbox/internal/actions"
)

func TestPolicy_Evaluate_MatchingAllow(t *testing.T) {
	pol := &Policy{
		Name:          "test",
		Version:       "1",
		DefaultEffect: EffectDeny,
		Rules: []Rule{
			{Action: "shell", Effect: EffectAllow, Description: "shell allowed"},
		},
	}

	action := actions.NewAction(actions.ActionTypeShell, "echo", nil)
	allowed, reason := pol.Evaluate(action)
	if !allowed {
		t.Errorf("expected allowed, got denied: %s", reason)
	}
	if reason != "shell allowed" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestPolicy_Evaluate_MatchingDeny(t *testing.T) {
	pol := &Policy{
		Name:          "test",
		Version:       "1",
		DefaultEffect: EffectAllow,
		Rules: []Rule{
			{Action: "network", Effect: EffectDeny, Description: "no network"},
		},
	}

	action := actions.NewAction(actions.ActionTypeNetworkRequest, "curl", nil)
	allowed, reason := pol.Evaluate(action)
	if allowed {
		t.Error("expected denied")
	}
	if reason != "no network" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestPolicy_Evaluate_WildcardRule(t *testing.T) {
	pol := &Policy{
		Name:    "allow-all",
		Version: "1",
		Rules: []Rule{
			{Action: "*", Effect: EffectAllow, Description: "everything allowed"},
		},
	}

	action := actions.NewAction(actions.ActionTypeCustom, "anything", nil)
	allowed, _ := pol.Evaluate(action)
	if !allowed {
		t.Error("expected allowed with wildcard rule")
	}
}

func TestPolicy_Evaluate_DefaultDeny(t *testing.T) {
	pol := &Policy{
		Name:          "strict",
		Version:       "1",
		DefaultEffect: EffectDeny,
		Rules:         []Rule{},
	}

	action := actions.NewAction(actions.ActionTypeShell, "something", nil)
	allowed, reason := pol.Evaluate(action)
	if allowed {
		t.Error("expected denied by default")
	}
	if reason != "denied by default (no matching rule)" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestPolicy_Evaluate_DefaultAllow(t *testing.T) {
	pol := &Policy{
		Name:          "permissive",
		Version:       "1",
		DefaultEffect: EffectAllow,
		Rules:         []Rule{},
	}

	action := actions.NewAction(actions.ActionTypeFileWrite, "write", nil)
	allowed, reason := pol.Evaluate(action)
	if !allowed {
		t.Error("expected allowed by default")
	}
	if reason != "allowed by default" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

func TestPolicy_Evaluate_FirstMatchWins(t *testing.T) {
	pol := &Policy{
		Name:    "first-match",
		Version: "1",
		Rules: []Rule{
			{Action: "shell", Effect: EffectDeny, Description: "deny shell first"},
			{Action: "shell", Effect: EffectAllow, Description: "allow shell second"},
		},
	}

	action := actions.NewAction(actions.ActionTypeShell, "cmd", nil)
	allowed, reason := pol.Evaluate(action)
	if allowed {
		t.Error("expected denied by first matching rule")
	}
	if reason != "deny shell first" {
		t.Errorf("unexpected reason: %s", reason)
	}
}

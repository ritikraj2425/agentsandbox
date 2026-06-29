package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ritikraj2425/agentsandbox/internal/actions"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// ─────────────────────────────────────────────────────────────────────────────
// matchesPrefix tests
// ─────────────────────────────────────────────────────────────────────────────

func TestMatchesPrefix_ExactMatch(t *testing.T) {
	// "echo" should match "echo" exactly.
	if !matchesPrefix("echo", "echo") {
		t.Error("expected 'echo' to match prefix 'echo'")
	}
}

func TestMatchesPrefix_WithArgs(t *testing.T) {
	// "echo hello" should match prefix "echo" (prefix + space + args).
	if !matchesPrefix("echo hello", "echo") {
		t.Error("expected 'echo hello' to match prefix 'echo'")
	}
}

func TestMatchesPrefix_PartialWordRejected(t *testing.T) {
	// "echoserver" should NOT match prefix "echo" (no word boundary).
	if matchesPrefix("echoserver", "echo") {
		t.Error("'echoserver' should NOT match prefix 'echo' — partial word match")
	}
}

func TestMatchesPrefix_MultiWordPrefix(t *testing.T) {
	// "go test ./..." should match prefix "go test".
	if !matchesPrefix("go test ./...", "go test") {
		t.Error("expected 'go test ./...' to match prefix 'go test'")
	}
}

func TestMatchesPrefix_NoMatch(t *testing.T) {
	// "npm install" should not match prefix "go test".
	if matchesPrefix("npm install", "go test") {
		t.Error("'npm install' should not match prefix 'go test'")
	}
}

func TestMatchesPrefix_DangerousCommand(t *testing.T) {
	// "rm -rf /" should match prefix "rm -rf".
	if !matchesPrefix("rm -rf /", "rm -rf") {
		t.Error("expected 'rm -rf /' to match prefix 'rm -rf'")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CommandPolicy.CheckCommand tests
// ─────────────────────────────────────────────────────────────────────────────

func newTestPolicy() *CommandPolicy {
	return &CommandPolicy{
		Name:        "test-policy",
		Description: "Unit test policy",
		Allow: []string{
			"go test",
			"gofmt",
			"echo",
			"cat",
		},
		Deny: []string{
			"rm -rf",
			"sudo",
			"chmod 777",
		},
		RequireApproval: []string{
			"npm install",
		},
	}
}

func TestCheckCommand_AllowedCommand(t *testing.T) {
	pol := newTestPolicy()

	decision := pol.CheckCommand("echo hello world")

	if !decision.Allowed {
		t.Errorf("expected allowed, got denied: %s", decision.Reason)
	}
	if decision.Effect != "allow" {
		t.Errorf("expected effect 'allow', got %q", decision.Effect)
	}
	if decision.MatchedRule != "echo" {
		t.Errorf("expected matched rule 'echo', got %q", decision.MatchedRule)
	}
}

func TestCheckCommand_AllowedMultiWordPrefix(t *testing.T) {
	pol := newTestPolicy()

	decision := pol.CheckCommand("go test ./... -v -count=1")

	if !decision.Allowed {
		t.Errorf("expected allowed, got denied: %s", decision.Reason)
	}
	if decision.MatchedRule != "go test" {
		t.Errorf("expected matched rule 'go test', got %q", decision.MatchedRule)
	}
}

func TestCheckCommand_DeniedCommand(t *testing.T) {
	pol := newTestPolicy()

	decision := pol.CheckCommand("rm -rf /tmp/test")

	if decision.Allowed {
		t.Error("expected denied, got allowed")
	}
	if decision.Effect != "deny" {
		t.Errorf("expected effect 'deny', got %q", decision.Effect)
	}
	if decision.MatchedRule != "rm -rf" {
		t.Errorf("expected matched rule 'rm -rf', got %q", decision.MatchedRule)
	}
}

func TestCheckCommand_DenyBeatsAllow(t *testing.T) {
	// Create a policy where "sudo echo" is both allowed (via "echo")
	// and denied (via "sudo"). Deny should always win.
	pol := &CommandPolicy{
		Allow: []string{"echo"},
		Deny:  []string{"sudo"},
	}

	decision := pol.CheckCommand("sudo echo hello")

	if decision.Allowed {
		t.Error("deny should beat allow — 'sudo echo' should be denied")
	}
	if decision.Effect != "deny" {
		t.Errorf("expected effect 'deny', got %q", decision.Effect)
	}
}

func TestCheckCommand_RequireApproval(t *testing.T) {
	pol := newTestPolicy()

	decision := pol.CheckCommand("npm install express")

	if decision.Allowed {
		t.Error("expected denied (require_approval), got allowed")
	}
	if decision.Effect != "require_approval" {
		t.Errorf("expected effect 'require_approval', got %q", decision.Effect)
	}
}

func TestCheckCommand_DefaultDeny(t *testing.T) {
	pol := newTestPolicy()

	// "wget" is not in any list — should be denied by default.
	decision := pol.CheckCommand("wget https://example.com")

	if decision.Allowed {
		t.Error("expected default deny, got allowed")
	}
	if decision.Effect != "default_deny" {
		t.Errorf("expected effect 'default_deny', got %q", decision.Effect)
	}
}

func TestCheckCommand_ExactPrefixOnly(t *testing.T) {
	pol := newTestPolicy()

	// "gofmtx" should NOT match "gofmt" (word boundary check).
	decision := pol.CheckCommand("gofmtx")

	if decision.Allowed {
		t.Error("'gofmtx' should not match 'gofmt' — partial word match")
	}
}

func TestCheckCommand_WhitespaceHandling(t *testing.T) {
	pol := newTestPolicy()

	// Leading/trailing whitespace should be trimmed before evaluation.
	decision := pol.CheckCommand("  echo hello  ")

	if !decision.Allowed {
		t.Errorf("expected allowed after trimming whitespace: %s", decision.Reason)
	}
}

func TestCheckCommand_EmptyPolicy(t *testing.T) {
	pol := &CommandPolicy{}

	// Everything should be denied by default when policy has no rules.
	decision := pol.CheckCommand("echo hello")

	if decision.Allowed {
		t.Error("expected default deny with empty policy")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// YAML file loading tests
// ─────────────────────────────────────────────────────────────────────────────

func TestLoadCommandPolicyFromFile_ValidFile(t *testing.T) {
	// Create a temporary policy YAML file.
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "test-policy.yaml")

	content := `# Test policy
name: test-coding
description: A test policy for unit tests

commands:
  allow:
    - go test
    - gofmt
    - echo
  deny:
    - rm -rf
    - sudo
  require_approval:
    - npm install
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test policy: %v", err)
	}

	pol, err := LoadCommandPolicyFromFile(policyPath)
	if err != nil {
		t.Fatalf("LoadCommandPolicyFromFile failed: %v", err)
	}

	// Verify metadata.
	if pol.Name != "test-coding" {
		t.Errorf("expected name 'test-coding', got %q", pol.Name)
	}

	// Verify allow list.
	if len(pol.Allow) != 3 {
		t.Fatalf("expected 3 allow rules, got %d", len(pol.Allow))
	}
	expectedAllow := []string{"go test", "gofmt", "echo"}
	for i, expected := range expectedAllow {
		if pol.Allow[i] != expected {
			t.Errorf("allow[%d]: expected %q, got %q", i, expected, pol.Allow[i])
		}
	}

	// Verify deny list.
	if len(pol.Deny) != 2 {
		t.Fatalf("expected 2 deny rules, got %d", len(pol.Deny))
	}

	// Verify require_approval list.
	if len(pol.RequireApproval) != 1 {
		t.Fatalf("expected 1 require_approval rule, got %d", len(pol.RequireApproval))
	}
	if pol.RequireApproval[0] != "npm install" {
		t.Errorf("expected 'npm install', got %q", pol.RequireApproval[0])
	}
}

func TestLoadCommandPolicyFromFile_FileNotFound(t *testing.T) {
	_, err := LoadCommandPolicyFromFile("/nonexistent/path/policy.yaml")

	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestLoadCommandPolicyFromFile_EndToEnd(t *testing.T) {
	// Write a policy, load it, and run CheckCommand — full integration.
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "policy.yaml")

	content := `name: e2e-test
commands:
  allow:
    - echo
    - cat
  deny:
    - rm -rf
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write policy: %v", err)
	}

	pol, err := LoadCommandPolicyFromFile(policyPath)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	// echo should be allowed.
	if d := pol.CheckCommand("echo hello"); !d.Allowed {
		t.Errorf("'echo hello' should be allowed: %s", d.Reason)
	}

	// rm -rf should be denied.
	if d := pol.CheckCommand("rm -rf /"); d.Allowed {
		t.Error("'rm -rf /' should be denied")
	}

	// wget should be default denied.
	if d := pol.CheckCommand("wget example.com"); d.Allowed {
		t.Error("'wget' should be default denied")
	}
}

func TestActionPolicy_AllowShellCommand(t *testing.T) {
	pol := &ActionPolicy{
		Name:               "structured",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeShellRun},
		Shell: ShellRules{
			AllowPrefixes: []string{"go test"},
		},
	}
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{"command": "go test ./..."})

	decision := pol.EvaluateAction(action, t.TempDir())

	if !decision.Allowed {
		t.Fatalf("expected allowed, got %s", decision.Reason)
	}
	if decision.Effect != string(EffectAllow) {
		t.Fatalf("expected allow, got %s", decision.Effect)
	}
}

func TestActionPolicy_DenyWinsOverAllow(t *testing.T) {
	pol := &ActionPolicy{
		Name:               "structured",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeShellRun},
		Shell: ShellRules{
			AllowPrefixes: []string{"rm"},
			DenyPrefixes:  []string{"rm -rf"},
		},
	}
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{"command": "rm -rf tmp"})

	decision := pol.EvaluateAction(action, t.TempDir())

	if decision.Allowed {
		t.Fatal("expected denied")
	}
	if decision.Effect != string(EffectDeny) {
		t.Fatalf("expected deny, got %s", decision.Effect)
	}
}

func TestActionPolicy_DefaultDeny(t *testing.T) {
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{"command": "echo hello"})

	decision := NewDefaultDenyActionPolicy().EvaluateAction(action, t.TempDir())

	if decision.Allowed {
		t.Fatal("expected default deny")
	}
	if decision.Effect != EffectDefaultDeny {
		t.Fatalf("expected default deny, got %s", decision.Effect)
	}
}

func TestActionPolicy_ApprovalRequired(t *testing.T) {
	pol := &ActionPolicy{
		Name:               "approval",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeShellRun},
		ApprovalRequired: ApprovalRules{
			ShellPrefixes: []string{"npm install"},
		},
	}
	action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{"command": "npm install express"})

	decision := pol.EvaluateAction(action, t.TempDir())

	if decision.Allowed {
		t.Fatal("approval-required action should not be allowed")
	}
	if decision.Effect != EffectRequireApproval {
		t.Fatalf("expected require approval, got %s", decision.Effect)
	}
}

func TestActionPolicy_MaxActionDuration(t *testing.T) {
	pol := &ActionPolicy{
		Name:               "duration",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeBrowserWaitFor},
		MaxActionDuration:  1000,
	}
	action := protocol.NewAction(protocol.ActionTypeBrowserWaitFor, map[string]interface{}{"timeout_ms": 2000})

	decision := pol.EvaluateAction(action, t.TempDir())

	if decision.Allowed {
		t.Fatal("expected duration denial")
	}
	if decision.MatchedRule != "max_action_duration_ms" {
		t.Fatalf("expected max duration rule, got %s", decision.MatchedRule)
	}
}

func TestActionPolicy_DeniesBrowserDomainUsingParsedURL(t *testing.T) {
	pol := &ActionPolicy{
		Name:               "browser",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeBrowserGoto},
		Browser: BrowserRules{
			AllowDomains: []string{"example.com"},
			DenyDomains:  []string{"blocked.example.com"},
		},
	}
	action := protocol.NewAction(protocol.ActionTypeBrowserGoto, map[string]interface{}{
		"url": "https://blocked.example.com:443/path",
	})

	decision := pol.EvaluateAction(action, t.TempDir())

	if decision.Allowed {
		t.Fatal("expected denied browser domain")
	}
	if decision.MatchedRule != "blocked.example.com" {
		t.Fatalf("expected blocked.example.com match, got %s", decision.MatchedRule)
	}
}

func TestActionPolicy_DeniesFileWorkspaceEscape(t *testing.T) {
	pol := &ActionPolicy{
		Name:               "files",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeFileRead},
		File: FileRules{
			AllowPaths: []string{"."},
		},
	}
	action := protocol.NewAction(protocol.ActionTypeFileRead, map[string]interface{}{
		"path": "../secret.txt",
	})

	decision := pol.EvaluateAction(action, t.TempDir())

	if decision.Allowed {
		t.Fatal("expected escaped path denied")
	}
	if decision.MatchedRule != "workspace_escape" {
		t.Fatalf("expected workspace_escape, got %s", decision.MatchedRule)
	}
}

func TestLoadActionPolicyFromFile(t *testing.T) {
	tmpDir := t.TempDir()
	policyPath := filepath.Join(tmpDir, "structured.yaml")
	content := `name: structured
default_effect: deny
max_action_duration_ms: 5000
action_types:
  allow:
    - shell.run
  deny:
    - browser.goto
shell:
  allow_prefixes:
    - go test
  deny_prefixes:
    - rm -rf
approval_required:
  shell_prefixes:
    - npm install
`
	if err := os.WriteFile(policyPath, []byte(content), 0644); err != nil {
		t.Fatalf("write policy: %v", err)
	}

	pol, err := LoadActionPolicyFromFile(policyPath)
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	if pol.Name != "structured" {
		t.Fatalf("expected name structured, got %s", pol.Name)
	}
	if len(pol.AllowedActionTypes) != 1 || pol.AllowedActionTypes[0] != protocol.ActionTypeShellRun {
		t.Fatalf("unexpected allowed action types: %#v", pol.AllowedActionTypes)
	}
	if pol.MaxActionDuration.Milliseconds() != 5000 {
		t.Fatalf("expected max duration 5000ms, got %d", pol.MaxActionDuration.Milliseconds())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Legacy Policy tests (backward compatibility)
// ─────────────────────────────────────────────────────────────────────────────

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

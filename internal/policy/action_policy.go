package policy

import (
	"bufio"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

const (
	EffectRequireApproval = "require_approval"
	EffectDefaultDeny     = "default_deny"
)

// ActionPolicy is the structured policy evaluated by the API gateway.
type ActionPolicy struct {
	Name               string
	Description        string
	DefaultEffect      Effect
	AllowedActionTypes []protocol.ActionType
	DeniedActionTypes  []protocol.ActionType
	Shell              ShellRules
	File               FileRules
	Browser            BrowserRules
	MaxActionDuration  time.Duration
	ApprovalRequired   ApprovalRules
}

type ShellRules struct {
	AllowPrefixes []string
	DenyPrefixes  []string
}

type FileRules struct {
	AllowPaths []string
	DenyPaths  []string
}

type BrowserRules struct {
	AllowDomains []string
	DenyDomains  []string
}

type ApprovalRules struct {
	ActionTypes    []protocol.ActionType
	ShellPrefixes  []string
	FilePaths      []string
	BrowserDomains []string
}

// NewDefaultDenyActionPolicy returns a policy that blocks every action.
func NewDefaultDenyActionPolicy() *ActionPolicy {
	return &ActionPolicy{
		Name:          "default-deny",
		DefaultEffect: EffectDeny,
	}
}

// EvaluateAction evaluates a protocol action against a structured policy.
func (p *ActionPolicy) EvaluateAction(action protocol.Action, workspace string) protocol.PolicyDecision {
	if p == nil {
		p = NewDefaultDenyActionPolicy()
	}

	actionType := string(action.Type)

	for _, denied := range p.DeniedActionTypes {
		if denied == action.Type {
			return p.decision(false, string(EffectDeny), actionType, "blocked by denied action type", nil)
		}
	}

	if decision, matched := p.evaluateDenyDimensions(action, workspace); matched {
		return decision
	}

	if decision, matched := p.evaluateApprovalRules(action, workspace); matched {
		return decision
	}

	if p.MaxActionDuration > 0 {
		if timeout, ok := actionTimeout(action); ok && timeout > p.MaxActionDuration {
			return p.decision(false, string(EffectDeny), "max_action_duration_ms", "blocked by max action duration", map[string]interface{}{
				"requested_timeout_ms":   timeout.Milliseconds(),
				"max_action_duration_ms": p.MaxActionDuration.Milliseconds(),
			})
		}
	}

	actionTypeAllowed := false
	for _, allowed := range p.AllowedActionTypes {
		if allowed == action.Type || allowed == protocol.ActionType("*") {
			actionTypeAllowed = true
			break
		}
	}
	if !actionTypeAllowed {
		return p.decision(false, EffectDefaultDeny, "", "denied by default: action type not allowed", map[string]interface{}{
			"type": actionType,
		})
	}

	if decision, ok := p.evaluateAllowDimensions(action, workspace); !ok {
		return decision
	}

	details := map[string]interface{}{"type": actionType}
	if p.MaxActionDuration > 0 {
		details["max_action_duration_ms"] = p.MaxActionDuration.Milliseconds()
	}
	return p.decision(true, string(EffectAllow), actionType, "allowed by policy", details)
}

func (p *ActionPolicy) evaluateDenyDimensions(action protocol.Action, workspace string) (protocol.PolicyDecision, bool) {
	switch action.Type {
	case protocol.ActionTypeShellRun:
		command := action.Command()
		for _, prefix := range p.Shell.DenyPrefixes {
			if matchesPrefix(strings.TrimSpace(command), prefix) {
				return p.decision(false, string(EffectDeny), prefix, "blocked by shell deny prefix", map[string]interface{}{"command": command}), true
			}
		}
	case protocol.ActionTypeFileRead, protocol.ActionTypeFileWrite, protocol.ActionTypeFilePatch:
		rel, err := normalizeActionPath(action, workspace)
		if err != nil {
			return p.decision(false, string(EffectDeny), "workspace_escape", err.Error(), nil), true
		}
		for _, denied := range p.File.DenyPaths {
			if pathMatches(rel, denied) {
				return p.decision(false, string(EffectDeny), denied, "blocked by file deny path", map[string]interface{}{"path": rel}), true
			}
		}
	case protocol.ActionTypeBrowserGoto:
		domain, err := actionDomain(action)
		if err != nil {
			return p.decision(false, string(EffectDeny), "invalid_url", err.Error(), nil), true
		}
		for _, denied := range p.Browser.DenyDomains {
			if domainMatches(domain, denied) {
				return p.decision(false, string(EffectDeny), denied, "blocked by browser deny domain", map[string]interface{}{"domain": domain}), true
			}
		}
	}
	return protocol.PolicyDecision{}, false
}

func (p *ActionPolicy) evaluateApprovalRules(action protocol.Action, workspace string) (protocol.PolicyDecision, bool) {
	for _, actionType := range p.ApprovalRequired.ActionTypes {
		if actionType == action.Type {
			return p.decision(false, EffectRequireApproval, string(actionType), "action requires approval", nil), true
		}
	}

	switch action.Type {
	case protocol.ActionTypeShellRun:
		command := action.Command()
		for _, prefix := range p.ApprovalRequired.ShellPrefixes {
			if matchesPrefix(strings.TrimSpace(command), prefix) {
				return p.decision(false, EffectRequireApproval, prefix, "shell command requires approval", map[string]interface{}{"command": command}), true
			}
		}
	case protocol.ActionTypeFileRead, protocol.ActionTypeFileWrite, protocol.ActionTypeFilePatch:
		rel, err := normalizeActionPath(action, workspace)
		if err != nil {
			return p.decision(false, string(EffectDeny), "workspace_escape", err.Error(), nil), true
		}
		for _, path := range p.ApprovalRequired.FilePaths {
			if pathMatches(rel, path) {
				return p.decision(false, EffectRequireApproval, path, "file path requires approval", map[string]interface{}{"path": rel}), true
			}
		}
	case protocol.ActionTypeBrowserGoto:
		domain, err := actionDomain(action)
		if err != nil {
			return p.decision(false, string(EffectDeny), "invalid_url", err.Error(), nil), true
		}
		for _, allowed := range p.ApprovalRequired.BrowserDomains {
			if domainMatches(domain, allowed) {
				return p.decision(false, EffectRequireApproval, allowed, "browser domain requires approval", map[string]interface{}{"domain": domain}), true
			}
		}
	}
	return protocol.PolicyDecision{}, false
}

func (p *ActionPolicy) evaluateAllowDimensions(action protocol.Action, workspace string) (protocol.PolicyDecision, bool) {
	switch action.Type {
	case protocol.ActionTypeShellRun:
		if len(p.Shell.AllowPrefixes) == 0 {
			return protocol.PolicyDecision{}, true
		}
		command := action.Command()
		for _, prefix := range p.Shell.AllowPrefixes {
			if matchesPrefix(strings.TrimSpace(command), prefix) {
				return protocol.PolicyDecision{}, true
			}
		}
		return p.decision(false, EffectDefaultDeny, "", "denied by default: shell command prefix not allowed", map[string]interface{}{"command": command}), false
	case protocol.ActionTypeFileRead, protocol.ActionTypeFileWrite, protocol.ActionTypeFilePatch:
		rel, err := normalizeActionPath(action, workspace)
		if err != nil {
			return p.decision(false, string(EffectDeny), "workspace_escape", err.Error(), nil), false
		}
		if len(p.File.AllowPaths) == 0 {
			return protocol.PolicyDecision{}, true
		}
		for _, allowed := range p.File.AllowPaths {
			if pathMatches(rel, allowed) {
				return protocol.PolicyDecision{}, true
			}
		}
		return p.decision(false, EffectDefaultDeny, "", "denied by default: file path not allowed", map[string]interface{}{"path": rel}), false
	case protocol.ActionTypeBrowserGoto:
		domain, err := actionDomain(action)
		if err != nil {
			return p.decision(false, string(EffectDeny), "invalid_url", err.Error(), nil), false
		}
		if len(p.Browser.AllowDomains) == 0 {
			return protocol.PolicyDecision{}, true
		}
		for _, allowed := range p.Browser.AllowDomains {
			if domainMatches(domain, allowed) {
				return protocol.PolicyDecision{}, true
			}
		}
		return p.decision(false, EffectDefaultDeny, "", "denied by default: browser domain not allowed", map[string]interface{}{"domain": domain}), false
	}
	return protocol.PolicyDecision{}, true
}

func (p *ActionPolicy) decision(allowed bool, effect string, matchedRule string, reason string, details map[string]interface{}) protocol.PolicyDecision {
	return protocol.PolicyDecision{
		Allowed:     allowed,
		Effect:      effect,
		PolicyName:  p.Name,
		MatchedRule: matchedRule,
		Reason:      reason,
		Details:     details,
	}
}

func normalizeActionPath(action protocol.Action, workspace string) (string, error) {
	raw, _ := action.Parameters["path"].(string)
	if raw == "" {
		return "", fmt.Errorf("%s requires a path parameter", action.Type)
	}
	if workspace == "" {
		workspace = "."
	}
	absWorkspace, err := filepath.Abs(workspace)
	if err != nil {
		return "", fmt.Errorf("cannot resolve workspace: %w", err)
	}

	var target string
	if filepath.IsAbs(raw) {
		target = filepath.Clean(raw)
	} else {
		target = filepath.Clean(filepath.Join(absWorkspace, raw))
	}

	rel, err := filepath.Rel(absWorkspace, target)
	if err != nil {
		return "", fmt.Errorf("cannot resolve file path: %w", err)
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." || filepath.IsAbs(rel) {
		return "", fmt.Errorf("file path escapes session workspace")
	}
	return filepath.ToSlash(filepath.Clean(rel)), nil
}

func actionDomain(action protocol.Action) (string, error) {
	raw := action.URL()
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid browser URL")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("browser URL scheme must be http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("invalid browser URL host")
	}
	return strings.ToLower(strings.TrimSuffix(host, ".")), nil
}

func pathMatches(path, policyPath string) bool {
	p := cleanPolicyPath(policyPath)
	if p == "." || p == "" {
		return true
	}
	path = filepath.ToSlash(filepath.Clean(path))
	return path == p || strings.HasPrefix(path, p+"/")
}

func cleanPolicyPath(path string) string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "."
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func domainMatches(domain, policyDomain string) bool {
	policyDomain = strings.ToLower(strings.TrimSpace(strings.TrimSuffix(policyDomain, ".")))
	if policyDomain == "" {
		return false
	}
	if policyDomain == "*" {
		return true
	}
	if ip := net.ParseIP(domain); ip != nil {
		return domain == policyDomain
	}
	return domain == policyDomain || strings.HasSuffix(domain, "."+policyDomain)
}

func actionTimeout(action protocol.Action) (time.Duration, bool) {
	raw, ok := action.Parameters["timeout_ms"]
	if !ok {
		return 0, false
	}
	switch v := raw.(type) {
	case int:
		return time.Duration(v) * time.Millisecond, true
	case int64:
		return time.Duration(v) * time.Millisecond, true
	case float64:
		return time.Duration(v) * time.Millisecond, true
	default:
		return 0, false
	}
}

// LoadActionPolicyFromFile loads the simple structured YAML format used in
// policies/*.yaml. It intentionally supports only the policy subset we ship.
func LoadActionPolicyFromFile(path string) (*ActionPolicy, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("cannot open policy file %s: %w", path, err)
	}
	defer file.Close()

	p := NewDefaultDenyActionPolicy()
	var section string
	var list string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || trimmed == ">" {
			continue
		}
		if strings.HasPrefix(trimmed, "- ") {
			value := strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")), `"'`)
			appendPolicyValue(p, section, list, value)
			continue
		}
		if !strings.Contains(trimmed, ":") {
			continue
		}

		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		parts := strings.SplitN(trimmed, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)

		if indent == 0 {
			list = ""
			switch key {
			case "name":
				p.Name = value
			case "description":
				p.Description = value
			case "default_effect":
				p.DefaultEffect = Effect(value)
			case "max_action_duration_ms":
				if ms, err := strconv.Atoi(value); err == nil && ms > 0 {
					p.MaxActionDuration = time.Duration(ms) * time.Millisecond
				}
			default:
				section = key
			}
			continue
		}

		if indent == 2 {
			if value != "" {
				appendPolicyValue(p, section, key, value)
			} else {
				list = key
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading policy file %s: %w", path, err)
	}
	if p.DefaultEffect == "" {
		p.DefaultEffect = EffectDeny
	}
	return p, nil
}

func appendPolicyValue(p *ActionPolicy, section string, list string, value string) {
	switch section {
	case "action_types":
		switch list {
		case "allow", "allowed":
			p.AllowedActionTypes = append(p.AllowedActionTypes, protocol.ActionType(value))
		case "deny", "denied":
			p.DeniedActionTypes = append(p.DeniedActionTypes, protocol.ActionType(value))
		}
	case "shell":
		switch list {
		case "allow_prefixes":
			p.Shell.AllowPrefixes = append(p.Shell.AllowPrefixes, value)
		case "deny_prefixes":
			p.Shell.DenyPrefixes = append(p.Shell.DenyPrefixes, value)
		}
	case "file":
		switch list {
		case "allow_paths":
			p.File.AllowPaths = append(p.File.AllowPaths, value)
		case "deny_paths":
			p.File.DenyPaths = append(p.File.DenyPaths, value)
		}
	case "browser":
		switch list {
		case "allow_domains":
			p.Browser.AllowDomains = append(p.Browser.AllowDomains, value)
		case "deny_domains":
			p.Browser.DenyDomains = append(p.Browser.DenyDomains, value)
		}
	case "approval_required":
		switch list {
		case "action_types":
			p.ApprovalRequired.ActionTypes = append(p.ApprovalRequired.ActionTypes, protocol.ActionType(value))
		case "shell_prefixes":
			p.ApprovalRequired.ShellPrefixes = append(p.ApprovalRequired.ShellPrefixes, value)
		case "file_paths":
			p.ApprovalRequired.FilePaths = append(p.ApprovalRequired.FilePaths, value)
		case "browser_domains":
			p.ApprovalRequired.BrowserDomains = append(p.ApprovalRequired.BrowserDomains, value)
		}
	}
}

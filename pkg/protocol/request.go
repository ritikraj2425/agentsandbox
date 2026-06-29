package protocol

import "fmt"

// ActionExecutionRequest is the public request payload accepted by
// POST /v1/sessions/:id/actions.
//
// New clients should send Type and Parameters. Command is preserved for
// backward compatibility with the original command-string API.
type ActionExecutionRequest struct {
	Type           ActionType             `json:"type,omitempty"`
	Parameters     map[string]interface{} `json:"parameters,omitempty"`
	ClientActionID string                 `json:"client_action_id,omitempty"`
	Command        string                 `json:"command,omitempty"`
}

// WorkspaceInitRequest configures optional session workspace initialization.
type WorkspaceInitRequest struct {
	Type      string `json:"type,omitempty"`
	GitURL    string `json:"git_url,omitempty"`
	ArchiveID string `json:"archive_id,omitempty"`
}

// ActionRequestError describes why an action execution request is invalid.
type ActionRequestError struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (e *ActionRequestError) Error() string {
	return e.Message
}

// ToAction validates the request and converts it into the runtime Action type.
// If Type is present, the structured action path is used. If Type is omitted,
// Command is parsed with the legacy command parser.
func (r ActionExecutionRequest) ToAction() (Action, *ActionRequestError) {
	if r.Type != "" {
		if err := ValidateAction(r.Type, r.Parameters); err != nil {
			return Action{}, err
		}
		action := NewAction(r.Type, r.Parameters)
		if r.ClientActionID != "" {
			action.ID = r.ClientActionID
		}
		return action, nil
	}

	if r.Command != "" {
		action := ParseCommand(r.Command)
		if r.ClientActionID != "" {
			action.ID = r.ClientActionID
		}
		return action, nil
	}

	return Action{}, invalidAction("missing_action", "request must include either type or command", nil)
}

// ValidateAction verifies that parameters contain the required fields for a
// supported structured action. It intentionally does not enforce policy.
func ValidateAction(actionType ActionType, params map[string]interface{}) *ActionRequestError {
	switch actionType {
	case ActionTypeShellRun:
		return requireString(actionType, params, "command")
	case ActionTypeFileRead:
		return requireString(actionType, params, "path")
	case ActionTypeFileWrite:
		if err := requireString(actionType, params, "path"); err != nil {
			return err
		}
		return requireString(actionType, params, "content")
	case ActionTypeFilePatch:
		if err := requireString(actionType, params, "path"); err != nil {
			return err
		}
		return requireString(actionType, params, "patch")
	case ActionTypeBrowserGoto:
		return requireString(actionType, params, "url")
	case ActionTypeBrowserClick:
		if hasString(params, "selector") {
			return nil
		}
		if hasNumber(params, "x") && hasNumber(params, "y") {
			return nil
		}
		return invalidParameter(actionType, "browser.click requires either selector string or x/y numbers", map[string]interface{}{
			"required": []string{"selector", "x", "y"},
		})
	case ActionTypeBrowserType:
		return requireString(actionType, params, "text")
	case ActionTypeBrowserPress:
		return requireString(actionType, params, "key")
	case ActionTypeBrowserWaitFor:
		if hasString(params, "selector") || hasString(params, "text") || hasNumber(params, "timeout_ms") {
			return nil
		}
		return invalidParameter(actionType, "browser.wait_for requires selector, text, or timeout_ms", map[string]interface{}{
			"required_any": []string{"selector", "text", "timeout_ms"},
		})
	case ActionTypeBrowserScreenshot:
		if _, ok := params["full_page"]; ok && !hasBool(params, "full_page") {
			return invalidParameter(actionType, "browser.screenshot full_page must be a boolean", map[string]interface{}{
				"field": "full_page",
			})
		}
		return nil
	case ActionTypeBrowserEvaluate:
		return requireString(actionType, params, "expression")
	case ActionTypeBrowserAssert:
		if err := requireString(actionType, params, "type"); err != nil {
			return err
		}
		if _, ok := params["expected"]; !ok {
			return invalidParameter(actionType, "browser.assert requires expected value", map[string]interface{}{
				"field": "expected",
			})
		}
		return nil
	case ActionTypeBrowserUserHandoff:
		if _, ok := params["message"]; ok && !hasString(params, "message") {
			return invalidParameter(actionType, "browser.user_handoff message must be a string", map[string]interface{}{
				"field": "message",
			})
		}
		if _, ok := params["ttl_seconds"]; ok && !hasNumber(params, "ttl_seconds") {
			return invalidParameter(actionType, "browser.user_handoff ttl_seconds must be a number", map[string]interface{}{
				"field": "ttl_seconds",
			})
		}
		return nil
	case ActionTypeTaskDone:
		if _, ok := params["summary"]; ok && !hasString(params, "summary") {
			return invalidParameter(actionType, "task.done summary must be a string", map[string]interface{}{
				"field": "summary",
			})
		}
		return nil
	default:
		return invalidAction("unsupported_action_type", fmt.Sprintf("unsupported action type: %s", actionType), map[string]interface{}{
			"type": string(actionType),
		})
	}
}

func requireString(actionType ActionType, params map[string]interface{}, field string) *ActionRequestError {
	if hasString(params, field) {
		return nil
	}
	return invalidParameter(actionType, fmt.Sprintf("%s requires parameters.%s string", actionType, field), map[string]interface{}{
		"field": field,
	})
}

func hasString(params map[string]interface{}, field string) bool {
	v, ok := params[field]
	if !ok {
		return false
	}
	_, ok = v.(string)
	return ok
}

func hasNumber(params map[string]interface{}, field string) bool {
	v, ok := params[field]
	if !ok {
		return false
	}
	_, ok = toFloat64(v)
	return ok
}

func hasBool(params map[string]interface{}, field string) bool {
	v, ok := params[field]
	if !ok {
		return false
	}
	_, ok = v.(bool)
	return ok
}

func invalidParameter(actionType ActionType, message string, details map[string]interface{}) *ActionRequestError {
	if details == nil {
		details = map[string]interface{}{}
	}
	details["type"] = string(actionType)
	return invalidAction("invalid_action_parameters", message, details)
}

func invalidAction(code string, message string, details map[string]interface{}) *ActionRequestError {
	return &ActionRequestError{
		Code:    code,
		Message: message,
		Details: details,
	}
}

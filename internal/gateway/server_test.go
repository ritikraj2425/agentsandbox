package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/policy"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

func TestAuthMiddleware(t *testing.T) {
	handler := AuthMiddleware("secret", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("No Auth Header", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", rec.Code)
		}
	})

	t.Run("Invalid Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("Expected 401, got %d", rec.Code)
		}
	})

	t.Run("Valid Token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer secret")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rec.Code)
		}
	})
}

func TestServer_CreateSession(t *testing.T) {
	server := NewServer(8080, 10, "secret", t.TempDir(), "", nil)

	reqBody, _ := json.Marshal(CreateSessionRequest{
		Backend: "local",
		TTL:     1 * time.Minute,
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBuffer(reqBody))
	req.Header.Set("Authorization", "Bearer secret")

	rec := httptest.NewRecorder()

	// Create a mock multiplexer to test the route
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/sessions", server.rateLimiter.Middleware(
		AuthMiddleware(server.authKey, server.handleCreateSession)))

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("Expected 201 Created, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp CreateSessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp.SessionID == "" {
		t.Error("Expected non-empty session ID")
	}
}

func TestServer_RunAction_LegacyCommand(t *testing.T) {
	server, rt, sessionID := newActionTestServer(t, "local", allowAllTestPolicy())

	body, _ := json.Marshal(RunActionRequest{Command: "echo legacy"})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if rt.action.Type != protocol.ActionTypeShellRun {
		t.Fatalf("expected shell.run, got %s", rt.action.Type)
	}
	if rt.action.Command() != "echo legacy" {
		t.Fatalf("expected legacy command to reach runtime, got %q", rt.action.Command())
	}
}

func TestServer_RunAction_StructuredShellRunReachesRuntime(t *testing.T) {
	server, rt, sessionID := newActionTestServer(t, "local", allowAllTestPolicy())

	body, _ := json.Marshal(RunActionRequest{
		Type: protocol.ActionTypeShellRun,
		Parameters: map[string]interface{}{
			"command": "echo structured",
		},
		ClientActionID: "client_shell_1",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if rt.action.ID != "client_shell_1" {
		t.Fatalf("expected client action id, got %s", rt.action.ID)
	}
	if rt.action.Type != protocol.ActionTypeShellRun {
		t.Fatalf("expected shell.run, got %s", rt.action.Type)
	}
	if rt.action.Command() != "echo structured" {
		t.Fatalf("expected structured command to reach runtime, got %q", rt.action.Command())
	}
}

func TestServer_RunAction_StructuredBrowserGotoReachesRuntime(t *testing.T) {
	server, rt, sessionID := newActionTestServer(t, "browser", allowAllTestPolicy())

	body, _ := json.Marshal(RunActionRequest{
		Type: protocol.ActionTypeBrowserGoto,
		Parameters: map[string]interface{}{
			"url": "https://example.com",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if rt.action.Type != protocol.ActionTypeBrowserGoto {
		t.Fatalf("expected browser.goto, got %s", rt.action.Type)
	}
	if rt.action.URL() != "https://example.com" {
		t.Fatalf("expected browser URL to reach runtime, got %q", rt.action.URL())
	}
}

func TestServer_RunAction_InvalidStructuredActionReturnsJSON400(t *testing.T) {
	server, _, sessionID := newActionTestServer(t, "local", allowAllTestPolicy())

	body, _ := json.Marshal(RunActionRequest{
		Type:       protocol.ActionTypeShellRun,
		Parameters: map[string]interface{}{},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected JSON content type, got %q", got)
	}

	var resp errorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if resp.Error.Code != "invalid_action_parameters" {
		t.Fatalf("expected invalid_action_parameters, got %s", resp.Error.Code)
	}
	if resp.Error.Message == "" {
		t.Fatal("expected useful error message")
	}
	if resp.Error.Details["field"] != "command" {
		t.Fatalf("expected field detail for command, got %#v", resp.Error.Details)
	}
}

func TestServer_RunAction_PolicyDeniedShellNeverReachesRuntime(t *testing.T) {
	server, rt, sessionID := newActionTestServer(t, "local", &policy.ActionPolicy{
		Name:               "deny-rm",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeShellRun},
		Shell: policy.ShellRules{
			AllowPrefixes: []string{"rm"},
			DenyPrefixes:  []string{"rm -rf"},
		},
	})

	body, _ := json.Marshal(RunActionRequest{
		Type: protocol.ActionTypeShellRun,
		Parameters: map[string]interface{}{
			"command": "rm -rf tmp",
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rt.called {
		t.Fatal("denied shell command reached runtime")
	}
	var obs protocol.Observation
	if err := json.NewDecoder(rec.Body).Decode(&obs); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	if obs.Status != protocol.ObsStatusDenied {
		t.Fatalf("expected denied, got %s", obs.Status)
	}
	if obs.PolicyDecision == nil || obs.PolicyDecision.Effect != string(policy.EffectDeny) {
		t.Fatalf("expected deny policy decision, got %#v", obs.PolicyDecision)
	}
}

func TestServer_RunAction_PolicyDeniedBrowserDomainNeverReachesRuntime(t *testing.T) {
	server, rt, sessionID := newActionTestServer(t, "browser", &policy.ActionPolicy{
		Name:               "browser-domains",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeBrowserGoto},
		Browser: policy.BrowserRules{
			AllowDomains: []string{"example.com"},
			DenyDomains:  []string{"blocked.example.com"},
		},
	})

	body, _ := json.Marshal(RunActionRequest{
		Type:       protocol.ActionTypeBrowserGoto,
		Parameters: map[string]interface{}{"url": "https://blocked.example.com/path"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rt.called {
		t.Fatal("denied browser domain reached runtime")
	}
	var obs protocol.Observation
	if err := json.NewDecoder(rec.Body).Decode(&obs); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	if obs.Status != protocol.ObsStatusDenied {
		t.Fatalf("expected denied, got %s", obs.Status)
	}
}

func TestServer_RunAction_PolicyDeniedFileEscapeNeverReachesRuntime(t *testing.T) {
	server, rt, sessionID := newActionTestServer(t, "local", &policy.ActionPolicy{
		Name:               "workspace-only",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeFileRead},
		File: policy.FileRules{
			AllowPaths: []string{"."},
		},
	})

	body, _ := json.Marshal(RunActionRequest{
		Type:       protocol.ActionTypeFileRead,
		Parameters: map[string]interface{}{"path": "../secret.txt"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rt.called {
		t.Fatal("workspace-escaping file action reached runtime")
	}
	var obs protocol.Observation
	if err := json.NewDecoder(rec.Body).Decode(&obs); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	if obs.Status != protocol.ObsStatusDenied {
		t.Fatalf("expected denied, got %s", obs.Status)
	}
}

func TestServer_RunAction_PolicyDefaultDeny(t *testing.T) {
	server, rt, sessionID := newActionTestServer(t, "local", policy.NewDefaultDenyActionPolicy())

	body, _ := json.Marshal(RunActionRequest{
		Type:       protocol.ActionTypeShellRun,
		Parameters: map[string]interface{}{"command": "echo nope"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rt.called {
		t.Fatal("default denied action reached runtime")
	}
	var obs protocol.Observation
	if err := json.NewDecoder(rec.Body).Decode(&obs); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	if obs.PolicyDecision == nil || obs.PolicyDecision.Effect != policy.EffectDefaultDeny {
		t.Fatalf("expected default deny decision, got %#v", obs.PolicyDecision)
	}
}

func TestServer_RunAction_PolicyApprovalRequired(t *testing.T) {
	server, rt, sessionID := newActionTestServer(t, "local", &policy.ActionPolicy{
		Name:               "approval",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeShellRun},
		ApprovalRequired: policy.ApprovalRules{
			ShellPrefixes: []string{"npm install"},
		},
	})

	body, _ := json.Marshal(RunActionRequest{
		Type:       protocol.ActionTypeShellRun,
		Parameters: map[string]interface{}{"command": "npm install express"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sessionID+"/actions", bytes.NewBuffer(body))
	rec := httptest.NewRecorder()

	server.handleRunAction(rec, req, sessionID)

	if rt.called {
		t.Fatal("approval-required action reached runtime")
	}
	var obs protocol.Observation
	if err := json.NewDecoder(rec.Body).Decode(&obs); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	if obs.Status != protocol.ObsStatusWaitingForApproval {
		t.Fatalf("expected waiting_for_approval, got %s", obs.Status)
	}
	if obs.PolicyDecision == nil || obs.PolicyDecision.Effect != policy.EffectRequireApproval {
		t.Fatalf("expected approval decision, got %#v", obs.PolicyDecision)
	}
}

func newActionTestServer(t *testing.T, runtimeName string, pol *policy.ActionPolicy) (*Server, *recordingRuntime, string) {
	t.Helper()

	server := NewServer(8080, 10, "secret", t.TempDir(), "", nil)
	rt := &recordingRuntime{name: runtimeName}
	sess, err := server.sessionManager.CreateSession(rt, time.Minute, pol)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	return server, rt, sess.ID
}

func allowAllTestPolicy() *policy.ActionPolicy {
	return &policy.ActionPolicy{
		Name:               "allow-all-test",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionType("*")},
	}
}

type recordingRuntime struct {
	name   string
	action protocol.Action
	called bool
}

func (r *recordingRuntime) Name() string {
	return r.name
}

func (r *recordingRuntime) Run(ctx context.Context, action protocol.Action) (protocol.Observation, error) {
	r.called = true
	r.action = action
	obs := protocol.NewObservation(action.ID)
	obs.Backend = r.name
	obs.Status = protocol.ObsStatusCompleted
	return obs, nil
}

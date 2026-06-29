package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	server, rt, sessionID := newActionTestServer(t, "local")

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
	server, rt, sessionID := newActionTestServer(t, "local")

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
	server, rt, sessionID := newActionTestServer(t, "browser")

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
	server, _, sessionID := newActionTestServer(t, "local")

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

func newActionTestServer(t *testing.T, runtimeName string) (*Server, *recordingRuntime, string) {
	t.Helper()

	server := NewServer(8080, 10, "secret", t.TempDir(), "", nil)
	rt := &recordingRuntime{name: runtimeName}
	sess, err := server.sessionManager.CreateSession(rt, time.Minute)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	return server, rt, sess.ID
}

type recordingRuntime struct {
	name   string
	action protocol.Action
}

func (r *recordingRuntime) Name() string {
	return r.name
}

func (r *recordingRuntime) Run(ctx context.Context, action protocol.Action) (protocol.Observation, error) {
	r.action = action
	obs := protocol.NewObservation(action.ID)
	obs.Backend = r.name
	obs.Status = protocol.ObsStatusCompleted
	return obs, nil
}

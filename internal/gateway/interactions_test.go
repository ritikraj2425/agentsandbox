package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/policy"
	"github.com/ritikraj2425/agentsandbox/internal/workspace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

func TestInteractionManagerScopedTokenExpires(t *testing.T) {
	mgr := NewInteractionManager("secret")
	interaction := mgr.Create("session-a", "example.test", time.Nanosecond)
	time.Sleep(time.Millisecond)

	if _, ok := mgr.Get(interaction.ID, interaction.Token); ok {
		t.Fatal("expected expired interaction token to be rejected")
	}
}

func TestInteractionManagerTokenOnlyAccessesOneSession(t *testing.T) {
	mgr := NewInteractionManager("secret")
	a := mgr.Create("session-a", "example.test", time.Minute)
	b := mgr.Create("session-b", "example.test", time.Minute)

	if _, ok := mgr.Get(b.ID, a.Token); ok {
		t.Fatal("token for session-a accessed session-b interaction")
	}
}

func TestInteractionEndpointsCreateGetComplete(t *testing.T) {
	server := NewServer(8080, 10, "secret", t.TempDir(), "", nil)
	ws, err := server.sessionManager.WorkspaceManager().Create(time.Minute, workspace.InitSpec{})
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	rt := &streamRuntime{name: "browser"}
	sess, err := server.sessionManager.CreateSession(rt, time.Minute, &policy.ActionPolicy{
		Name:               "allow",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionType("*")},
	}, ws)
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sess.ID+"/interactions", strings.NewReader(`{"ttl_seconds":60}`))
	req.Host = "api.test"
	rec := httptest.NewRecorder()
	server.handleCreateInteraction(rec, req, sess.ID)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected create 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var created Interaction
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatalf("decode interaction: %v", err)
	}
	if created.Token == "" || created.StreamURL == "" {
		t.Fatalf("expected token and stream URL, got %#v", created)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/interactions/"+created.ID+"?token="+created.Token, nil)
	getReq.Host = "api.test"
	getRec := httptest.NewRecorder()
	server.handleInteractionRoute(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("expected get 200, got %d: %s", getRec.Code, getRec.Body.String())
	}

	completeReq := httptest.NewRequest(http.MethodPost, "/v1/interactions/"+created.ID+"/complete?token="+created.Token, nil)
	completeRec := httptest.NewRecorder()
	server.handleInteractionRoute(completeRec, completeReq)
	if completeRec.Code != http.StatusOK {
		t.Fatalf("expected complete 200, got %d: %s", completeRec.Code, completeRec.Body.String())
	}

	againReq := httptest.NewRequest(http.MethodGet, "/v1/interactions/"+created.ID+"?token="+created.Token, nil)
	againRec := httptest.NewRecorder()
	server.handleInteractionRoute(againRec, againReq)
	if againRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected completed interaction to be unavailable, got %d", againRec.Code)
	}
}

func TestBrowserUserHandoffActionReturnsWaitingForUser(t *testing.T) {
	server := NewServer(8080, 10, "secret", t.TempDir(), "", nil)
	ws, err := server.sessionManager.WorkspaceManager().Create(time.Minute, workspace.InitSpec{})
	if err != nil {
		t.Fatalf("workspace: %v", err)
	}
	sess, err := server.sessionManager.CreateSession(&streamRuntime{name: "browser"}, time.Minute, &policy.ActionPolicy{
		Name:               "allow-handoff",
		AllowedActionTypes: []protocol.ActionType{protocol.ActionTypeBrowserUserHandoff},
	}, ws)
	if err != nil {
		t.Fatalf("session: %v", err)
	}

	body := `{"type":"browser.user_handoff","parameters":{"message":"please finish login","ttl_seconds":60}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions/"+sess.ID+"/actions", strings.NewReader(body))
	req.Host = "api.test"
	rec := httptest.NewRecorder()
	server.handleRunAction(rec, req, sess.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var obs protocol.Observation
	if err := json.NewDecoder(rec.Body).Decode(&obs); err != nil {
		t.Fatalf("decode observation: %v", err)
	}
	if obs.Status != protocol.ObsStatusWaitingForUser {
		t.Fatalf("expected waiting_for_user, got %s", obs.Status)
	}
	if obs.BrowserMetadata == nil || obs.BrowserMetadata.UserHandoff == nil || obs.BrowserMetadata.UserHandoff.URL == "" {
		t.Fatalf("expected user handoff link, got %#v", obs.BrowserMetadata)
	}
}

func TestAttachArtifactURLs(t *testing.T) {
	server := NewServer(8080, 10, "secret", t.TempDir(), "", nil)
	ref := protocol.ArtifactRef{ID: "screenshot.png", URL: "/artifacts/screenshot.png"}
	obs := protocol.Observation{
		Artifacts: []protocol.ArtifactRef{ref},
		BrowserMetadata: &protocol.BrowserMetadata{
			ScreenshotArtifact: &ref,
		},
	}

	server.attachArtifactURLs("session-1", &obs)

	want := "/v1/sessions/session-1/artifacts/screenshot.png"
	if obs.Artifacts[0].URL != want {
		t.Fatalf("expected artifact URL %s, got %s", want, obs.Artifacts[0].URL)
	}
	if obs.BrowserMetadata.ScreenshotArtifact.URL != want {
		t.Fatalf("expected screenshot artifact URL %s, got %s", want, obs.BrowserMetadata.ScreenshotArtifact.URL)
	}
}

type streamRuntime struct {
	name string
}

func (r *streamRuntime) Name() string {
	return r.name
}

func (r *streamRuntime) Run(ctx context.Context, action protocol.Action) (protocol.Observation, error) {
	obs := protocol.NewObservation(action.ID)
	obs.Status = protocol.ObsStatusCompleted
	return obs, nil
}

func (r *streamRuntime) StreamURL(host string) string {
	return "ws://" + host + ":6080"
}

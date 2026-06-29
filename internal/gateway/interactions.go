package gateway

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/trace"
)

type Interaction struct {
	ID        string    `json:"interaction_id"`
	SessionID string    `json:"session_id"`
	Token     string    `json:"token,omitempty"`
	StreamURL string    `json:"stream_url"`
	ExpiresAt time.Time `json:"expires_at"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
}

type InteractionManager struct {
	mu           sync.RWMutex
	secret       string
	interactions map[string]*Interaction
}

func NewInteractionManager(secret string) *InteractionManager {
	if secret == "" {
		secret = randomHex(32)
	}
	return &InteractionManager{
		secret:       secret,
		interactions: make(map[string]*Interaction),
	}
}

func (m *InteractionManager) Create(sessionID string, host string, ttl time.Duration) *Interaction {
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	now := time.Now().UTC()
	id := randomHex(8)
	expiresAt := now.Add(ttl)
	token := m.sign(id, sessionID, expiresAt)
	interaction := &Interaction{
		ID:        id,
		SessionID: sessionID,
		Token:     token,
		StreamURL: fmt.Sprintf("http://%s/v1/interactions/%s?token=%s", host, id, token),
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}

	m.mu.Lock()
	m.interactions[id] = interaction
	m.mu.Unlock()
	return interaction
}

func (m *InteractionManager) Get(id string, token string) (*Interaction, bool) {
	m.mu.RLock()
	interaction, ok := m.interactions[id]
	m.mu.RUnlock()
	if !ok || interaction.Completed || time.Now().UTC().After(interaction.ExpiresAt) {
		return nil, false
	}
	if !m.verify(id, interaction.SessionID, interaction.ExpiresAt, token) {
		return nil, false
	}
	return interaction, true
}

func (m *InteractionManager) Complete(id string, token string) (*Interaction, bool) {
	interaction, ok := m.Get(id, token)
	if !ok {
		return nil, false
	}
	m.mu.Lock()
	interaction.Completed = true
	m.mu.Unlock()
	return interaction, true
}

func (m *InteractionManager) sign(id string, sessionID string, expiresAt time.Time) string {
	payload := fmt.Sprintf("%s.%s.%d", id, sessionID, expiresAt.Unix())
	mac := hmac.New(sha256.New, []byte(m.secret))
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig
}

func (m *InteractionManager) verify(id string, sessionID string, expiresAt time.Time, token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return false
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}
	expectedPayload := fmt.Sprintf("%s.%s.%d", id, sessionID, expiresAt.Unix())
	if string(payloadBytes) != expectedPayload {
		return false
	}
	expected := m.sign(id, sessionID, expiresAt)
	return hmac.Equal([]byte(expected), []byte(token))
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

type streamProvider interface {
	StreamURL(host string) string
}

func (s *Server) handleCreateInteraction(w http.ResponseWriter, r *http.Request, sessionID string) {
	sess, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	if _, ok := sess.Runtime.(streamProvider); !ok {
		http.Error(w, "Browser stream not supported for this session", http.StatusBadRequest)
		return
	}

	var req struct {
		TTLSeconds int `json:"ttl_seconds,omitempty"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	interaction := s.interactions.Create(sessionID, r.Host, time.Duration(req.TTLSeconds)*time.Second)
	if sess.Logger != nil {
		sess.Logger.LogEvent(trace.EventTypeHumanInteraction, "User browser handoff created", map[string]interface{}{
			"interaction_id": interaction.ID,
			"expires_at":     interaction.ExpiresAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(interaction)
}

func (s *Server) handleInteractionRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/interactions/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Interaction ID required", http.StatusBadRequest)
		return
	}
	id := parts[0]
	token := r.URL.Query().Get("token")
	if token == "" {
		token = r.Header.Get("X-AgentSandbox-Interaction-Token")
	}

	if len(parts) == 1 && r.Method == http.MethodGet {
		s.handleGetInteraction(w, r, id, token)
		return
	}
	if len(parts) == 2 && parts[1] == "complete" && r.Method == http.MethodPost {
		s.handleCompleteInteraction(w, r, id, token)
		return
	}
	http.Error(w, "Not Found", http.StatusNotFound)
}

func (s *Server) handleGetInteraction(w http.ResponseWriter, r *http.Request, id string, token string) {
	interaction, ok := s.interactions.Get(id, token)
	if !ok {
		http.Error(w, "Interaction expired or invalid", http.StatusUnauthorized)
		return
	}
	sess, err := s.sessionManager.GetSession(interaction.SessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	sp, ok := sess.Runtime.(streamProvider)
	if !ok || sp.StreamURL(r.Host) == "" {
		http.Error(w, "Browser stream not available", http.StatusBadRequest)
		return
	}
	if sess.Logger != nil {
		sess.Logger.LogEvent(trace.EventTypeHumanInteraction, "User browser stream opened", map[string]interface{}{
			"interaction_id": id,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"interaction_id": id,
		"session_id":     interaction.SessionID,
		"stream_url":     sp.StreamURL(r.Host),
		"expires_at":     interaction.ExpiresAt,
	})
}

func (s *Server) handleCompleteInteraction(w http.ResponseWriter, r *http.Request, id string, token string) {
	interaction, ok := s.interactions.Complete(id, token)
	if !ok {
		http.Error(w, "Interaction expired or invalid", http.StatusUnauthorized)
		return
	}
	if sess, err := s.sessionManager.GetSession(interaction.SessionID); err == nil && sess.Logger != nil {
		sess.Logger.LogEvent(trace.EventTypeHumanInteraction, "User browser handoff completed", map[string]interface{}{
			"interaction_id": id,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"completed":      true,
		"interaction_id": id,
	})
}

package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/runtime"
)

var (
	ErrMaxSessionsExceeded = errors.New("maximum concurrent sessions exceeded")
	ErrSessionNotFound     = errors.New("session not found")
)

// Session represents a virtual, multi-tenant sandbox environment.
type Session struct {
	ID        string
	Runtime   runtime.Runtime
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager tracks active sessions and enforces resource quotas.
type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	maxSessions int
}

// NewSessionManager creates a new manager with the specified concurrent limit.
func NewSessionManager(maxSessions int) *SessionManager {
	return &SessionManager{
		sessions:    make(map[string]*Session),
		maxSessions: maxSessions,
	}
}

// CreateSession registers a new sandbox session if quotas allow.
// The caller is responsible for constructing the actual runtime backend.
func (sm *SessionManager) CreateSession(rt runtime.Runtime, ttl time.Duration) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Enforce global max session limit
	if len(sm.sessions) >= sm.maxSessions {
		return nil, ErrMaxSessionsExceeded
	}

	id := generateSessionID()
	now := time.Now()

	sess := &Session{
		ID:        id,
		Runtime:   rt,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}

	sm.sessions[id] = sess
	return sess, nil
}

// GetSession retrieves an active session by its ID.
func (sm *SessionManager) GetSession(id string) (*Session, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	sess, ok := sm.sessions[id]
	if !ok {
		return nil, ErrSessionNotFound
	}
	// Check expiration
	if time.Now().After(sess.ExpiresAt) {
		return nil, ErrSessionNotFound
	}
	return sess, nil
}

// DeleteSession removes a session from tracking.
func (sm *SessionManager) DeleteSession(id string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.sessions, id)
}

// CleanupLoop runs in the background and removes expired sessions.
func (sm *SessionManager) CleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for id, sess := range sm.sessions {
			if now.After(sess.ExpiresAt) {
				delete(sm.sessions, id)
			}
		}
		sm.mu.Unlock()
	}
}

// generateSessionID creates a unique 16-character hex string.
func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

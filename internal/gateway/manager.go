package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"sync"
	"time"

	"github.com/ritikraj2425/agentsandbox/internal/policy"
	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
)

var (
	ErrMaxSessionsExceeded = errors.New("maximum concurrent sessions exceeded")
	ErrSessionNotFound     = errors.New("session not found")
)

// Session represents a virtual, multi-tenant sandbox environment.
type Session struct {
	ID        string
	Runtime   runtime.Runtime
	Policy    *policy.ActionPolicy
	Logger    *trace.RunLogger
	CreatedAt time.Time
	ExpiresAt time.Time
}

// SessionManager tracks active sessions and enforces resource quotas.
type SessionManager struct {
	mu          sync.RWMutex
	sessions    map[string]*Session
	maxSessions int
	workDir     string
}

// NewSessionManager creates a new manager with the specified concurrent limit.
func NewSessionManager(maxSessions int, workDir string) *SessionManager {
	return &SessionManager{
		sessions:    make(map[string]*Session),
		maxSessions: maxSessions,
		workDir:     workDir,
	}
}

// CreateSession registers a new sandbox session if quotas allow.
// The caller is responsible for constructing the actual runtime backend.
func (sm *SessionManager) CreateSession(rt runtime.Runtime, ttl time.Duration, pol *policy.ActionPolicy) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Enforce global max session limit
	if len(sm.sessions) >= sm.maxSessions {
		return nil, ErrMaxSessionsExceeded
	}

	id := generateSessionID()
	now := time.Now()

	logger, err := trace.NewRunLogger(sm.workDir)
	if err != nil {
		return nil, err
	}

	sess := &Session{
		ID:        id,
		Runtime:   rt,
		Policy:    pol,
		Logger:    logger,
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
	if sess, ok := sm.sessions[id]; ok {
		if sess.Logger != nil {
			sess.Logger.Close()
		}
		delete(sm.sessions, id)
	}
}

// CleanupLoop runs in the background and removes expired sessions.
func (sm *SessionManager) CleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for id, sess := range sm.sessions {
			if now.After(sess.ExpiresAt) {
				if sess.Logger != nil {
					sess.Logger.Close()
				}
				delete(sm.sessions, id)
			}
		}
		sm.mu.Unlock()
	}
}

// ListSessions returns all non-expired sessions.
func (sm *SessionManager) ListSessions() []*Session {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	now := time.Now()
	var sessions []*Session
	for _, sess := range sm.sessions {
		if now.Before(sess.ExpiresAt) {
			sessions = append(sessions, sess)
		}
	}
	return sessions
}

// generateSessionID creates a unique 16-character hex string.
func generateSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

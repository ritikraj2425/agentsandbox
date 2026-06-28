package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
	// Import backends to register them
	dockerrt "github.com/ritikraj2425/agentsandbox/runtimes/docker"
	firecrackerrt "github.com/ritikraj2425/agentsandbox/runtimes/firecracker"
	gvisorrt "github.com/ritikraj2425/agentsandbox/runtimes/gvisor"
	localrt "github.com/ritikraj2425/agentsandbox/runtimes/local"
	browserrt "github.com/ritikraj2425/agentsandbox/runtimes/browser"
)

// ensure backends are imported
var _ = dockerrt.New
var _ = localrt.New
var _ = gvisorrt.New
var _ = firecrackerrt.New

// Server represents the API Gateway HTTP server.
type Server struct {
	port           int
	authKey        string
	sessionManager *SessionManager
	rateLimiter    *RateLimiter
}

// NewServer creates a new API Gateway Server instance.
func NewServer(port int, maxSessions int, authKey string) *Server {
	return &Server{
		port:           port,
		authKey:        authKey,
		sessionManager: NewSessionManager(maxSessions),
		// 10 requests per second with a burst of 20
		rateLimiter: NewRateLimiter(rate.Limit(10), 20),
	}
}

// Start boots up the HTTP server and blocks.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Wrap handlers in RateLimit and Auth middleware
	mux.HandleFunc("/v1/sessions", s.rateLimiter.Middleware(
		AuthMiddleware(s.authKey, s.handleCreateSession)))
	mux.HandleFunc("/v1/sessions/", s.rateLimiter.Middleware(
		AuthMiddleware(s.authKey, s.handleSessionRoute)))

	// Start background cleanup
	go s.sessionManager.CleanupLoop(1 * time.Minute)

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting Multi-Tenant API Gateway on %s", addr)
	return http.ListenAndServe(addr, mux)
}

// handleSessionRoute routes requests to specific session sub-paths.
func (s *Server) handleSessionRoute(w http.ResponseWriter, r *http.Request) {
	// Path is /v1/sessions/<id> or /v1/sessions/<id>/actions
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/sessions/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}
	sessionID := parts[0]

	if len(parts) == 1 && r.Method == http.MethodDelete {
		s.handleDeleteSession(w, r, sessionID)
		return
	}

	if len(parts) == 2 && parts[1] == "actions" && r.Method == http.MethodPost {
		s.handleRunAction(w, r, sessionID)
		return
	}

	if len(parts) == 2 && parts[1] == "stream" {
		HandleWebSocketStream(s.sessionManager, w, r, sessionID)
		return
	}

	http.Error(w, "Not Found", http.StatusNotFound)
}

type CreateSessionRequest struct {
	Backend string        `json:"backend"`
	Image   string        `json:"image,omitempty"`
	CPUs    string        `json:"cpus,omitempty"`
	Memory  string        `json:"memory,omitempty"`
	TTL     time.Duration `json:"ttl"`
}

type CreateSessionResponse struct {
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	if req.TTL == 0 {
		req.TTL = 1 * time.Hour // default 1 hour
	}

	workDir, _ := os.Getwd()

	var rt runtime.Runtime
	var err error

	switch req.Backend {
	case "local":
		rt = localrt.New(workDir, nil)
	case "docker":
		rt, err = dockerrt.New(workDir, req.Image, nil)
	case "gvisor":
		rt, err = gvisorrt.New(workDir, req.Image, req.CPUs, req.Memory, nil)
	case "firecracker":
		rt, err = firecrackerrt.New(firecrackerrt.Config{
			WorkDir: workDir,
		})
	case "browser":
		rt, err = browserrt.New(browserrt.Config{
			WorkDir: workDir,
		})
	default:
		// Attempt registry
		factory, exists := runtime.Registry[req.Backend]
		if !exists {
			http.Error(w, fmt.Sprintf("Unknown backend: %s", req.Backend), http.StatusBadRequest)
			return
		}
		rt, err = factory(workDir)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to initialize backend: %s", err), http.StatusInternalServerError)
		return
	}

	sess, err := s.sessionManager.CreateSession(rt, req.TTL)
	if err != nil {
		if err == ErrMaxSessionsExceeded {
			http.Error(w, err.Error(), http.StatusTooManyRequests)
		} else {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	resp := CreateSessionResponse{
		SessionID: sess.ID,
		ExpiresAt: sess.ExpiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleDeleteSession(w http.ResponseWriter, r *http.Request, sessionID string) {
	_, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	s.sessionManager.DeleteSession(sessionID)
	w.WriteHeader(http.StatusNoContent)
}

type RunActionRequest struct {
	Command string `json:"command"`
}

func (s *Server) handleRunAction(w http.ResponseWriter, r *http.Request, sessionID string) {
	sess, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var req RunActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	action := protocol.ParseCommand(req.Command)

	// Execute via runtime
	obs, err := sess.Runtime.Run(context.Background(), action)
	if err != nil {
		// Even if error, observation might hold partial state
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obs)
}

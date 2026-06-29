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

	"github.com/ritikraj2425/agentsandbox/internal/observe"
	"github.com/ritikraj2425/agentsandbox/internal/runtime"
	"github.com/ritikraj2425/agentsandbox/internal/trace"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
	// Import backends to register them
	browserrt "github.com/ritikraj2425/agentsandbox/runtimes/browser"
	dockerrt "github.com/ritikraj2425/agentsandbox/runtimes/docker"
	firecrackerrt "github.com/ritikraj2425/agentsandbox/runtimes/firecracker"
	gvisorrt "github.com/ritikraj2425/agentsandbox/runtimes/gvisor"
	localrt "github.com/ritikraj2425/agentsandbox/runtimes/local"
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
	eventBus       *observe.EventBus
	workDir        string
	corsOrigin     string
}

// NewServer creates a new API Gateway Server instance.
func NewServer(port int, maxSessions int, authKey string, workDir string, corsOrigin string) *Server {
	return &Server{
		port:           port,
		authKey:        authKey,
		sessionManager: NewSessionManager(maxSessions, workDir),
		// 10 requests per second with a burst of 20
		rateLimiter: NewRateLimiter(rate.Limit(10), 20),
		eventBus:    observe.NewEventBus(),
		workDir:     workDir,
		corsOrigin:  corsOrigin,
	}
}

// Start boots up the HTTP server and blocks.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Existing API routes — wrapped in RateLimit and Bearer Auth middleware.
	mux.HandleFunc("/v1/sessions", s.rateLimiter.Middleware(
		AuthMiddleware(s.authKey, s.handleCreateSession)))
	mux.HandleFunc("/v1/sessions/", s.rateLimiter.Middleware(
		AuthMiddleware(s.authKey, s.handleSessionRoute)))

	// --- Dashboard Auth routes ---
	mux.HandleFunc("/v1/auth/login", s.handleLogin)
	mux.Handle("/v1/auth/me", JWTAuthMiddleware(s.authKey, http.HandlerFunc(s.handleMe)))

	// --- Dashboard Runs routes ---
	mux.Handle("/v1/runs", JWTAuthMiddleware(s.authKey, http.HandlerFunc(s.handleListRuns)))
	mux.Handle("/v1/runs/", JWTAuthMiddleware(s.authKey, http.HandlerFunc(s.handleRunRoute)))

	// --- Dashboard Sessions routes ---
	mux.Handle("/v1/dashboard/sessions", JWTAuthMiddleware(s.authKey, http.HandlerFunc(s.handleListActiveSessions)))
	mux.Handle("/v1/dashboard/sessions/", JWTAuthMiddleware(s.authKey, http.HandlerFunc(s.handleDashboardSessionRoute)))

	// Start background cleanup
	go s.sessionManager.CleanupLoop(1 * time.Minute)

	// Optionally wrap the entire mux with CORS middleware.
	var handler http.Handler = mux
	if s.corsOrigin != "" {
		handler = CORSMiddleware(s.corsOrigin, mux)
	}

	addr := fmt.Sprintf(":%d", s.port)
	log.Printf("Starting Multi-Tenant API Gateway on %s", addr)
	return http.ListenAndServe(addr, handler)
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

// handleRunRoute parses the run ID from the URL path and dispatches to handleGetRun.
func (s *Server) handleRunRoute(w http.ResponseWriter, r *http.Request) {
	runID := strings.TrimPrefix(r.URL.Path, "/v1/runs/")
	if runID == "" {
		http.Error(w, "Run ID required", http.StatusBadRequest)
		return
	}
	s.handleGetRun(w, r, runID)
}

// handleDashboardSessionRoute routes dashboard session sub-paths for VNC and events.
func (s *Server) handleDashboardSessionRoute(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/v1/dashboard/sessions/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Session ID required", http.StatusBadRequest)
		return
	}
	sessionID := parts[0]

	if len(parts) == 2 && parts[1] == "vnc" {
		s.handleGetVNCEndpoint(w, r, sessionID)
		return
	}
	if len(parts) == 2 && parts[1] == "events" {
		s.handleSessionEvents(w, r, sessionID)
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

	if sess.Logger != nil {
		sess.Logger.LogEvent(trace.EventTypeActionReceived, "Action received", map[string]interface{}{
			"command": req.Command,
		})
		sess.Logger.LogEvent(trace.EventTypeProcessStarted, "Executing action", nil)
	}

	// Publish action received
	s.eventBus.Publish(observe.SessionEvent{
		SessionID: sessionID,
		Type:      "action.received",
		Payload:   map[string]interface{}{"command": req.Command},
		Timestamp: time.Now().UTC(),
	})

	// Execute via runtime
	obs, err := sess.Runtime.Run(context.Background(), action)
	if err != nil {
		if sess.Logger != nil {
			sess.Logger.LogEvent(trace.EventTypeError, "Execution failed", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	if sess.Logger != nil {
		if obs.StdoutSummary != "" {
			sess.Logger.WriteStdout(obs.StdoutSummary)
		}
		if obs.StderrSummary != "" {
			sess.Logger.WriteStderr(obs.StderrSummary)
		}
		
		eventData := map[string]interface{}{
			"exit_code":   obs.ExitCode,
			"duration_ms": obs.DurationMs,
		}
		if obs.Screenshot != "" {
			eventData["screenshot"] = obs.Screenshot
		}

		sess.Logger.LogEvent(trace.EventTypeProcessFinished, "Action finished", eventData)
	}

	// Publish process finished
	s.eventBus.Publish(observe.SessionEvent{
		SessionID: sessionID,
		Type:      "process.finished",
		Payload:   map[string]interface{}{"exit_code": obs.ExitCode, "duration_ms": obs.DurationMs},
		Timestamp: time.Now().UTC(),
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obs)
}

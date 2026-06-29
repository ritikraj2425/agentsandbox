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
	"github.com/ritikraj2425/agentsandbox/internal/store"
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

// Server handles HTTP/WebSocket communication for the sandbox.
type Server struct {
	port           int
	authKey        string // Keeping for backwards compatibility if needed, but db preferred
	corsOrigin     string
	router         *http.ServeMux
	sessionManager *SessionManager
	rateLimiter    *RateLimiter
	eventBus       *observe.EventBus
	dbStore        store.Store
	workDir        string
}

// NewServer creates a new API gateway server.
func NewServer(port int, maxSessions int, authKey string, workDir string, corsOrigin string, dbStore store.Store) *Server {
	eventBus := observe.NewEventBus()
	s := &Server{
		port:           port,
		authKey:        authKey,
		corsOrigin:     corsOrigin,
		router:         http.NewServeMux(),
		sessionManager: NewSessionManager(maxSessions, workDir),
		rateLimiter:    NewRateLimiter(rate.Limit(10), 20),
		eventBus:       eventBus,
		dbStore:        dbStore,
		workDir:        workDir,
	}
	return s
}

// Start boots up the HTTP server and blocks.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Existing API routes — wrapped in RateLimit and Bearer Auth middleware.
	mux.HandleFunc("/v1/sessions", s.rateLimiter.Middleware(
		s.authMiddleware(http.HandlerFunc(s.handleCreateSession)).ServeHTTP))
	mux.HandleFunc("/v1/sessions/", s.rateLimiter.Middleware(
		s.authMiddleware(http.HandlerFunc(s.handleSessionRoute)).ServeHTTP))

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

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Unauthorized: Missing Authorization header", http.StatusUnauthorized)
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Unauthorized: Invalid Authorization format", http.StatusUnauthorized)
			return
		}

		rawKey := parts[1]

		if s.dbStore != nil {
			// Database Authentication
			user, err := s.dbStore.ValidateAPIKey(r.Context(), rawKey)
			if err != nil {
				http.Error(w, "Unauthorized: Invalid or revoked API key", http.StatusUnauthorized)
				return
			}
			// Set user context if needed later
			r = r.WithContext(context.WithValue(r.Context(), "user", user))
		} else {
			// Legacy fallback for single static auth key
			if authHeader != "Bearer "+s.authKey {
				http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
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

	if s.dbStore != nil {
		userID := "legacy-admin"
		if u, ok := r.Context().Value("user").(*store.User); ok && u != nil {
			userID = u.ID
		}
		s.dbStore.CreateSession(r.Context(), store.SessionRecord{
			ID:        sess.ID,
			UserID:    userID,
			Backend:   req.Backend,
			Status:    "running",
			CreatedAt: sess.CreatedAt,
		})
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

	if s.dbStore != nil {
		s.dbStore.UpdateSessionStatus(r.Context(), sessionID, "completed")
	}

	w.WriteHeader(http.StatusNoContent)
}

type RunActionRequest = protocol.ActionExecutionRequest

type errorResponse struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (s *Server) handleRunAction(w http.ResponseWriter, r *http.Request, sessionID string) {
	sess, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var req RunActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_json", "Invalid JSON payload", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	action, actionErr := req.ToAction()
	if actionErr != nil {
		writeJSONError(w, http.StatusBadRequest, actionErr.Code, actionErr.Message, actionErr.Details)
		return
	}

	// Phase 2 policy hook: structured actions are normalized before execution.

	if sess.Logger != nil {
		sess.Logger.LogEvent(trace.EventTypeActionReceived, "Action received", map[string]interface{}{
			"action_id": action.ID,
			"type":      string(action.Type),
			"command":   req.Command,
		})
		sess.Logger.LogEvent(trace.EventTypeProcessStarted, "Executing action", nil)
	}

	// Publish action received
	s.eventBus.Publish(observe.SessionEvent{
		SessionID: sessionID,
		Type:      "action.received",
		Payload: map[string]interface{}{
			"action_id": action.ID,
			"type":      string(action.Type),
			"command":   req.Command,
		},
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

func writeJSONError(w http.ResponseWriter, status int, code string, message string, details map[string]interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(errorResponse{
		Error: errorBody{
			Code:    code,
			Message: message,
			Details: details,
		},
	})
}

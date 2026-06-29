package gateway

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ritikraj2425/agentsandbox/internal/replay"
)

var dashUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// CORSMiddleware handles CORS headers
func CORSMiddleware(allowOrigin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// createJWT generates a simple JWT (HMAC-SHA256)
func createJWT(secret string, expiry time.Duration) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now()
	payloadStr := fmt.Sprintf(`{"iat":%d,"exp":%d}`, now.Unix(), now.Add(expiry).Unix())
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadStr))
	msg := header + "." + payload

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return msg + "." + signature
}

// validateJWT verifies the simple JWT
func validateJWT(secret string, token string) bool {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false
	}
	msg := parts[0] + "." + parts[1]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	expectedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if parts[2] != expectedSignature {
		return false
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return false
	}

	expFloat, ok := payload["exp"].(float64)
	if !ok {
		return false
	}
	if time.Now().Unix() > int64(expFloat) {
		return false
	}

	return true
}

func JWTAuthMiddleware(secret string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string

		authHeader := r.Header.Get("Authorization")
		if strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		} else {
			cookie, err := r.Cookie("agentsandbox_token")
			if err == nil {
				token = cookie.Value
			}
		}

		if token == "" || !validateJWT(secret, token) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.APIKey != s.authKey {
		http.Error(w, "Invalid API Key", http.StatusUnauthorized)
		return
	}

	expiry := 24 * time.Hour
	token := createJWT(s.authKey, expiry)

	http.SetCookie(w, &http.Cookie{
		Name:     "agentsandbox_token",
		Value:    token,
		Expires:  time.Now().Add(expiry),
		HttpOnly: true,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"expires_at": time.Now().Add(expiry).Format(time.RFC3339),
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"authenticated": true,
	})
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	runs, err := replay.ListRuns(s.workDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request, runID string) {
	run, err := replay.LoadRun(s.workDir, runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

func (s *Server) handleListActiveSessions(w http.ResponseWriter, r *http.Request) {
	sessions := s.sessionManager.ListSessions()

	type SessionMeta struct {
		ID        string    `json:"id"`
		Backend   string    `json:"backend"`
		CreatedAt time.Time `json:"created_at"`
		ExpiresAt time.Time `json:"expires_at"`
		HasVNC    bool      `json:"has_vnc"`
	}

	var res []SessionMeta
	for _, sess := range sessions {
		hasVNC := false
		if sess.Runtime != nil && sess.Runtime.Name() == "browser" {
			hasVNC = true
		}
		res = append(res, SessionMeta{
			ID:        sess.ID,
			Backend:   sess.Runtime.Name(),
			CreatedAt: sess.CreatedAt,
			ExpiresAt: sess.ExpiresAt,
			HasVNC:    hasVNC,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

type vncProvider interface {
	VNCPort() string
}

func (s *Server) handleGetVNCEndpoint(w http.ResponseWriter, r *http.Request, sessionID string) {
	sess, err := s.sessionManager.GetSession(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	vp, ok := sess.Runtime.(vncProvider)
	if !ok || vp.VNCPort() == "" {
		http.Error(w, "VNC not supported for this session", http.StatusBadRequest)
		return
	}

	host := r.Host
	if strings.Contains(host, ":") {
		host = strings.Split(host, ":")[0]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"vnc_url": fmt.Sprintf("ws://%s:%s", host, vp.VNCPort()),
	})
}

func (s *Server) handleSessionEvents(w http.ResponseWriter, r *http.Request, sessionID string) {
	conn, err := dashUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ch, unsubscribe := s.eventBus.Subscribe(sessionID)
	defer unsubscribe()

	for {
		select {
		case ev := <-ch:
			if err := conn.WriteJSON(ev); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

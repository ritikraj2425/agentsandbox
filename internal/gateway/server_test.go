package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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
	server := NewServer(8080, 10, "secret")
	
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

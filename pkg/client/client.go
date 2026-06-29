// Package client provides a low-level Go HTTP client for the AgentSandbox API.
//
// This package handles raw HTTP communication with the API Gateway:
// constructing requests, setting authentication headers, parsing responses,
// and surfacing errors. It is intentionally low-level — the higher-level
// sdk package wraps this client with ergonomic session management.
//
// Usage:
//
//	c := client.New("http://localhost:8080", "secret_token")
//	resp, err := c.CreateSession(client.CreateSessionRequest{
//	    Backend: "docker",
//	    Image:   "golang:1.26",
//	})
//	obs, err := c.RunAction(resp.SessionID, client.RunActionRequest{
//	    Command: "echo hello",
//	})
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

// Client communicates with the AgentSandbox API Gateway over HTTP.
type Client struct {
	// BaseURL is the root URL of the gateway (e.g., "http://localhost:8080").
	BaseURL string

	// Token is the Bearer token used for authentication.
	Token string

	// HTTPClient is the underlying HTTP client. You can replace it with
	// a custom client to configure timeouts, TLS, proxies, etc.
	HTTPClient *http.Client
}

// New creates a new API Client targeting the given gateway URL and auth token.
func New(baseURL string, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTPClient: &http.Client{
			Timeout: 2 * time.Minute,
		},
	}
}

// --- Request/Response Types ---

// CreateSessionRequest matches the gateway's POST /v1/sessions payload.
type CreateSessionRequest struct {
	Backend string        `json:"backend"`
	Image   string        `json:"image,omitempty"`
	CPUs    string        `json:"cpus,omitempty"`
	Memory  string        `json:"memory,omitempty"`
	TTL     time.Duration `json:"ttl,omitempty"`
}

// CreateSessionResponse is returned by POST /v1/sessions.
type CreateSessionResponse struct {
	SessionID string    `json:"session_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// RunActionRequest matches the gateway's POST /v1/sessions/{id}/actions payload.
type RunActionRequest = protocol.ActionExecutionRequest

// APIError represents an error response from the API Gateway.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// --- API Methods ---

// CreateSession sends a POST /v1/sessions request to create a new sandbox session.
func (c *Client) CreateSession(req CreateSessionRequest) (*CreateSessionResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, c.BaseURL+"/v1/sessions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, c.readError(resp)
	}

	var result CreateSessionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &result, nil
}

// RunAction sends a POST /v1/sessions/{id}/actions request to execute a command.
func (c *Client) RunAction(sessionID string, req RunActionRequest) (*protocol.Observation, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/v1/sessions/%s/actions", c.BaseURL, sessionID)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, c.readError(resp)
	}

	var obs protocol.Observation
	if err := json.NewDecoder(resp.Body).Decode(&obs); err != nil {
		return nil, fmt.Errorf("failed to decode observation: %w", err)
	}
	return &obs, nil
}

// DeleteSession sends a DELETE /v1/sessions/{id} request to tear down a session.
func (c *Client) DeleteSession(sessionID string) error {
	url := fmt.Sprintf("%s/v1/sessions/%s", c.BaseURL, sessionID)
	httpReq, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	c.setHeaders(httpReq)

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return c.readError(resp)
	}
	return nil
}

// --- Internal Helpers ---

// setHeaders applies the standard authentication and content-type headers.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "agentsandbox-go-sdk/1.0")
}

// readError reads the response body and wraps it into an APIError.
func (c *Client) readError(resp *http.Response) error {
	body, _ := io.ReadAll(resp.Body)
	return &APIError{
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}
}

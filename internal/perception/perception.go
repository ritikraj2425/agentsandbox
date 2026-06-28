// Package perception handles agent observation of action results and
// environment state. It provides the CDP (Chrome DevTools Protocol) client
// that communicates with a headless Chromium instance to execute browser
// actions: navigation, clicking, typing, and screenshot capture.
//
// Architecture:
//
//	┌─────────────────────┐
//	│   Browser Runtime   │  (runtimes/browser)
//	│  Docker + Xvfb +    │
//	│  Chromium headless   │
//	└────────┬────────────┘
//	         │ WebSocket (CDP)
//	         ▼
//	┌─────────────────────┐
//	│   CDPClient         │  (this package)
//	│  Navigate / Click / │
//	│  Type / Screenshot  │
//	└─────────────────────┘
//
// The CDPClient connects to Chrome's DevTools Protocol endpoint via
// WebSocket and sends JSON-RPC commands. Each command is assigned a
// monotonically increasing ID, and the client reads responses until it
// finds the matching reply.
package perception

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// CDPClient communicates with a Chromium instance over the Chrome DevTools Protocol.
type CDPClient struct {
	conn    *websocket.Conn
	mu      sync.Mutex
	nextID  atomic.Int64
	timeout time.Duration
}

// cdpRequest is a JSON-RPC request sent to Chrome.
type cdpRequest struct {
	ID     int64                  `json:"id"`
	Method string                 `json:"method"`
	Params map[string]interface{} `json:"params,omitempty"`
}

// cdpResponse is a JSON-RPC response received from Chrome.
type cdpResponse struct {
	ID     int64                  `json:"id"`
	Result map[string]interface{} `json:"result,omitempty"`
	Error  *cdpError              `json:"error,omitempty"`
}

type cdpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// NewCDPClient connects to a Chrome DevTools Protocol WebSocket endpoint.
// The wsURL is typically "ws://localhost:9222/devtools/page/<id>".
func NewCDPClient(wsURL string, timeout time.Duration) (*CDPClient, error) {
	dialer := websocket.Dialer{
		HandshakeTimeout: timeout,
	}

	conn, _, err := dialer.Dial(wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to CDP endpoint %s: %w", wsURL, err)
	}

	return &CDPClient{
		conn:    conn,
		timeout: timeout,
	}, nil
}

// Close shuts down the WebSocket connection.
func (c *CDPClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// send dispatches a CDP command and waits for the matching response.
func (c *CDPClient) send(method string, params map[string]interface{}) (map[string]interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := c.nextID.Add(1)

	req := cdpRequest{
		ID:     id,
		Method: method,
		Params: params,
	}

	if err := c.conn.SetWriteDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set write deadline: %w", err)
	}

	if err := c.conn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("failed to send CDP command %s: %w", method, err)
	}

	// Read responses until we find the one matching our request ID.
	// Chrome may send event notifications interleaved with responses.
	if err := c.conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set read deadline: %w", err)
	}

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return nil, fmt.Errorf("failed to read CDP response for %s: %w", method, err)
		}

		var resp cdpResponse
		if err := json.Unmarshal(message, &resp); err != nil {
			continue // Skip malformed messages
		}

		// Skip event notifications (they have no ID or ID=0)
		if resp.ID != id {
			continue
		}

		if resp.Error != nil {
			return nil, fmt.Errorf("CDP error for %s: [%d] %s", method, resp.Error.Code, resp.Error.Message)
		}

		return resp.Result, nil
	}
}

// --- Public Browser Actions ---

// Navigate directs the browser to the given URL and waits for the page to load.
func (c *CDPClient) Navigate(url string) error {
	_, err := c.send("Page.navigate", map[string]interface{}{
		"url": url,
	})
	if err != nil {
		return fmt.Errorf("navigation failed: %w", err)
	}

	// Wait for the page to finish loading by polling
	time.Sleep(500 * time.Millisecond)
	return nil
}

// Click dispatches a mouse click at the given (x, y) coordinates.
// This simulates a full click: mousePressed followed by mouseReleased.
func (c *CDPClient) Click(x, y float64) error {
	// Mouse pressed
	_, err := c.send("Input.dispatchMouseEvent", map[string]interface{}{
		"type":       "mousePressed",
		"x":          x,
		"y":          y,
		"button":     "left",
		"clickCount": 1,
	})
	if err != nil {
		return fmt.Errorf("click mousePressed failed: %w", err)
	}

	// Mouse released
	_, err = c.send("Input.dispatchMouseEvent", map[string]interface{}{
		"type":       "mouseReleased",
		"x":          x,
		"y":          y,
		"button":     "left",
		"clickCount": 1,
	})
	if err != nil {
		return fmt.Errorf("click mouseReleased failed: %w", err)
	}

	return nil
}

// ClickSelector clicks on the first element matching the CSS selector.
// It uses JavaScript to find the element's bounding box, then dispatches
// a click at the center of that box.
func (c *CDPClient) ClickSelector(selector string) error {
	// Use JavaScript to find the element and get its position
	js := fmt.Sprintf(`
		(function() {
			var el = document.querySelector(%q);
			if (!el) return JSON.stringify({error: "element not found"});
			var rect = el.getBoundingClientRect();
			return JSON.stringify({x: rect.x + rect.width/2, y: rect.y + rect.height/2});
		})()
	`, selector)

	result, err := c.Evaluate(js)
	if err != nil {
		return fmt.Errorf("failed to locate selector %q: %w", selector, err)
	}

	// Parse the coordinate result
	var coords struct {
		X     float64 `json:"x"`
		Y     float64 `json:"y"`
		Error string  `json:"error,omitempty"`
	}
	if err := json.Unmarshal([]byte(result), &coords); err != nil {
		return fmt.Errorf("failed to parse element coordinates: %w", err)
	}
	if coords.Error != "" {
		return fmt.Errorf("selector %q: %s", selector, coords.Error)
	}

	return c.Click(coords.X, coords.Y)
}

// TypeText types the given text by dispatching individual key events.
// For each character, it sends keyDown, char, and keyUp events.
func (c *CDPClient) TypeText(text string) error {
	for _, ch := range text {
		charStr := string(ch)

		// keyDown
		_, err := c.send("Input.dispatchKeyEvent", map[string]interface{}{
			"type": "keyDown",
			"text": charStr,
		})
		if err != nil {
			return fmt.Errorf("keyDown failed for %q: %w", charStr, err)
		}

		// char
		_, err = c.send("Input.dispatchKeyEvent", map[string]interface{}{
			"type": "char",
			"text": charStr,
		})
		if err != nil {
			return fmt.Errorf("char event failed for %q: %w", charStr, err)
		}

		// keyUp
		_, err = c.send("Input.dispatchKeyEvent", map[string]interface{}{
			"type": "keyUp",
			"text": charStr,
		})
		if err != nil {
			return fmt.Errorf("keyUp failed for %q: %w", charStr, err)
		}
	}
	return nil
}

// CaptureScreenshot takes a PNG screenshot and returns it as a base64 string.
func (c *CDPClient) CaptureScreenshot() (string, error) {
	result, err := c.send("Page.captureScreenshot", map[string]interface{}{
		"format": "png",
	})
	if err != nil {
		return "", fmt.Errorf("screenshot capture failed: %w", err)
	}

	data, ok := result["data"].(string)
	if !ok {
		return "", fmt.Errorf("screenshot response missing base64 data")
	}

	return data, nil
}

// GetTitle returns the current page title.
func (c *CDPClient) GetTitle() (string, error) {
	result, err := c.Evaluate("document.title")
	if err != nil {
		return "", err
	}
	return result, nil
}

// GetURL returns the current page URL.
func (c *CDPClient) GetURL() (string, error) {
	result, err := c.Evaluate("window.location.href")
	if err != nil {
		return "", err
	}
	return result, nil
}

// Evaluate executes a JavaScript expression and returns the string result.
func (c *CDPClient) Evaluate(expression string) (string, error) {
	result, err := c.send("Runtime.evaluate", map[string]interface{}{
		"expression":    expression,
		"returnByValue": true,
	})
	if err != nil {
		return "", fmt.Errorf("evaluate failed: %w", err)
	}

	// Extract the value from the result object
	resultObj, ok := result["result"].(map[string]interface{})
	if !ok {
		return "", nil
	}

	value, ok := resultObj["value"]
	if !ok {
		return "", nil
	}

	switch v := value.(type) {
	case string:
		return v, nil
	default:
		b, _ := json.Marshal(v)
		return string(b), nil
	}
}

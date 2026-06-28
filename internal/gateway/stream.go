package gateway

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ritikraj2425/agentsandbox/pkg/protocol"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Allow all origins for testing; in prod, configure CORS properly
	CheckOrigin: func(r *http.Request) bool { return true },
}

// StreamMessage represents a message sent or received over the WebSocket.
type StreamMessage struct {
	Type    string      `json:"type"`              // "action", "stdout", "stderr", "observation", "error"
	Payload interface{} `json:"payload,omitempty"` // Depends on Type
}

// HandleWebSocketStream upgrades the HTTP connection to a WebSocket and handles interactive streaming.
func HandleWebSocketStream(sm *SessionManager, w http.ResponseWriter, r *http.Request, sessionID string) {
	sess, err := sm.GetSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Failed to upgrade to websocket: %v", err)
		return
	}
	defer conn.Close()

	// In a fully interactive stream, the runtime would write directly to the socket.
	// For now, we process Actions from the client, run them, and send the Observation back.
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var req RunActionRequest
		if err := json.Unmarshal(message, &req); err != nil {
			sendError(conn, "Invalid JSON payload")
			continue
		}

		action := protocol.NewAction(protocol.ActionTypeShellRun, map[string]interface{}{
			"command": req.Command,
		})

		// A real-time implementation would inject a writer here to stream stdout/stderr
		// directly from the exec.Cmd to the conn.
		// For this phase, we execute synchronously and return the final observation.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		obs, err := sess.Runtime.Run(ctx, action)
		cancel()

		if err != nil && obs.Status == "" {
			sendError(conn, err.Error())
			continue
		}

		// Send Observation
		resp := StreamMessage{
			Type:    "observation",
			Payload: obs,
		}
		if err := conn.WriteJSON(resp); err != nil {
			log.Printf("Failed to write to websocket: %v", err)
			break
		}
	}
}

func sendError(conn *websocket.Conn, msg string) {
	conn.WriteJSON(StreamMessage{
		Type:    "error",
		Payload: msg,
	})
}

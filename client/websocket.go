package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/websocket"
)

// WSConn wraps a WebSocket connection to a Daptin /live endpoint.
type WSConn struct {
	conn   *websocket.Conn
	nextID atomic.Int64
}

// DialWebSocket connects to the Daptin WebSocket endpoint, performs the
// handshake (reads session-open), and returns a ready-to-use connection.
func DialWebSocket(endpoint, authToken string) (*WSConn, error) {
	wsURL := httpToWS(endpoint) + "/live"
	origin := endpoint

	config, err := websocket.NewConfig(wsURL, origin)
	if err != nil {
		return nil, fmt.Errorf("websocket config: %w", err)
	}
	if authToken != "" {
		config.Header.Set("Authorization", "Bearer "+authToken)
	}

	conn, err := websocket.DialConfig(config)
	if err != nil {
		// The websocket library gives opaque "bad status" errors.
		// Do a preflight to surface the real HTTP status and body.
		if body, code, fetchErr := preflight(endpoint+"/live", authToken); fetchErr == nil && code != http.StatusSwitchingProtocols {
			return nil, fmt.Errorf("websocket upgrade rejected (HTTP %d): %s", code, body)
		}
		return nil, fmt.Errorf("websocket dial: %w", err)
	}

	ws := &WSConn{conn: conn}

	// Read and validate session-open message
	msg, err := ws.ReadMessage()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("reading session-open: %w", err)
	}
	if msg["type"] != "session" || msg["status"] != "open" {
		conn.Close()
		return nil, fmt.Errorf("expected session-open, got: %v", msg)
	}

	return ws, nil
}

// Send sends a method call with attributes and returns the request ID.
func (ws *WSConn) Send(method string, attrs map[string]interface{}) (string, error) {
	id := strconv.FormatInt(ws.nextID.Add(1), 10)
	msg := map[string]interface{}{
		"id":         id,
		"method":     method,
		"attributes": attrs,
	}
	err := websocket.JSON.Send(ws.conn, msg)
	if err != nil {
		return "", fmt.Errorf("websocket send: %w", err)
	}
	return id, nil
}

// SendPing sends a keepalive ping.
func (ws *WSConn) SendPing() error {
	return websocket.JSON.Send(ws.conn, map[string]interface{}{"method": "ping"})
}

// ReadMessage reads one JSON message from the connection.
func (ws *WSConn) ReadMessage() (map[string]interface{}, error) {
	var msg map[string]interface{}
	err := websocket.JSON.Receive(ws.conn, &msg)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// WaitResponse reads messages until it finds a response matching the given ID.
// Non-response messages (events, pongs) are passed to the handler if non-nil.
func (ws *WSConn) WaitResponse(id string, eventHandler func(map[string]interface{})) (map[string]interface{}, error) {
	for {
		msg, err := ws.ReadMessage()
		if err != nil {
			return nil, err
		}
		if msg["type"] == "response" && msg["id"] == id {
			if ok, _ := msg["ok"].(bool); !ok {
				errMsg, _ := msg["error"].(string)
				return msg, fmt.Errorf("server error: %s", errMsg)
			}
			return msg, nil
		}
		if eventHandler != nil {
			eventHandler(msg)
		}
	}
}

// WaitResponseTimeout is like WaitResponse but returns nil if no response
// arrives within the timeout. This is needed for fire-and-forget methods
// like new-message where the server only responds on error.
func (ws *WSConn) WaitResponseTimeout(id string, timeout time.Duration) (map[string]interface{}, error) {
	ws.conn.SetReadDeadline(time.Now().Add(timeout))
	defer ws.conn.SetReadDeadline(time.Time{})

	for {
		msg, err := ws.ReadMessage()
		if err != nil {
			// Timeout means no error response — success
			if isTimeout(err) {
				return nil, nil
			}
			return nil, err
		}
		if msg["type"] == "response" && msg["id"] == id {
			if ok, _ := msg["ok"].(bool); !ok {
				errMsg, _ := msg["error"].(string)
				return msg, fmt.Errorf("server error: %s", errMsg)
			}
			return msg, nil
		}
	}
}

func isTimeout(err error) bool {
	type netTimeout interface {
		Timeout() bool
	}
	if t, ok := err.(netTimeout); ok {
		return t.Timeout()
	}
	return false
}

// ReadMessageTimeout reads one JSON message with a deadline.
// Returns nil, nil on timeout.
func (ws *WSConn) ReadMessageTimeout(timeout time.Duration) (map[string]interface{}, error) {
	ws.conn.SetReadDeadline(time.Now().Add(timeout))
	defer ws.conn.SetReadDeadline(time.Time{})
	msg, err := ws.ReadMessage()
	if err != nil && isTimeout(err) {
		return nil, nil
	}
	return msg, err
}

// Close closes the WebSocket connection.
func (ws *WSConn) Close() error {
	return ws.conn.Close()
}

// EventToJSONLine serializes a message to a compact JSON line.
func EventToJSONLine(msg map[string]interface{}) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// preflight makes a regular HTTP GET to the endpoint to retrieve
// the actual status code and body when WebSocket upgrade fails.
func preflight(url, authToken string) (string, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", 0, err
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return strings.TrimSpace(string(body)), resp.StatusCode, nil
}

func httpToWS(endpoint string) string {
	if strings.HasPrefix(endpoint, "https://") {
		return "wss://" + strings.TrimPrefix(endpoint, "https://")
	}
	return "ws://" + strings.TrimPrefix(endpoint, "http://")
}

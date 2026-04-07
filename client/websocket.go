package client

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
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

	header := http.Header{}
	header.Set("Origin", endpoint)
	if authToken != "" {
		header.Set("Authorization", "Bearer "+authToken)
	}

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}
	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
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
	err := ws.conn.WriteJSON(msg)
	if err != nil {
		return "", fmt.Errorf("websocket send: %w", err)
	}
	return id, nil
}

// SendPing sends a keepalive ping.
func (ws *WSConn) SendPing() error {
	return ws.conn.WriteJSON(map[string]interface{}{"method": "ping"})
}

// ReadMessage reads one JSON message from the connection.
func (ws *WSConn) ReadMessage() (map[string]interface{}, error) {
	_, data, err := ws.conn.ReadMessage()
	if err != nil {
		return nil, err
	}
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
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

// DecodeResponseData extracts the "data" field from a WS response.
// The server sends Data as jsoniter.RawMessage ([]byte), which the
// standard JSON encoder serializes as base64. This function handles
// both base64-encoded strings and already-parsed map values.
func DecodeResponseData(resp map[string]interface{}) map[string]interface{} {
	switch d := resp["data"].(type) {
	case map[string]interface{}:
		return d
	case string:
		var m map[string]interface{}
		if json.Unmarshal([]byte(d), &m) == nil {
			return m
		}
		decoded, err := base64.StdEncoding.DecodeString(d)
		if err == nil {
			if json.Unmarshal(decoded, &m) == nil {
				return m
			}
		}
	}
	return nil
}

// EventToJSONLine serializes a message to a compact JSON line.
func EventToJSONLine(msg map[string]interface{}) (string, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func httpToWS(endpoint string) string {
	if strings.HasPrefix(endpoint, "https://") {
		return "wss://" + strings.TrimPrefix(endpoint, "https://")
	}
	return "ws://" + strings.TrimPrefix(endpoint, "http://")
}

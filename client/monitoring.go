package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (e *ExtendedClient) Health() (map[string]interface{}, error) {
	started := time.Now()
	resp, err := e.nextRequest().SetDebug(false).Get(e.Endpoint + "/api/world?page%5Bnumber%5D=1&page%5Bsize%5D=1")
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}
	var envelope map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &envelope); err != nil {
		return nil, fmt.Errorf("health response is not valid JSON: %w", err)
	}
	if _, ok := envelope["data"].([]interface{}); !ok {
		return nil, fmt.Errorf("health response is malformed: expected JSON:API data array")
	}
	return map[string]interface{}{
		"healthy":     true,
		"status":      http.StatusText(resp.StatusCode()),
		"status_code": resp.StatusCode(),
		"latency_ms":  time.Since(started).Milliseconds(),
	}, nil
}

func (e *ExtendedClient) Statistics() (map[string]interface{}, error) {
	if e.AuthToken == "" {
		return nil, fmt.Errorf("statistics requires an authenticated context")
	}
	resp, err := e.nextRequest().SetDebug(false).Get(e.Endpoint + "/statistics")
	if err := e.checkResponse(resp, err); err != nil {
		return nil, err
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &payload); err != nil {
		return nil, fmt.Errorf("statistics response is not valid JSON: %w", err)
	}
	if dangerous := findDangerousMonitoringField(payload, ""); dangerous != "" {
		return nil, fmt.Errorf("statistics response contains unsafe field %q; refusing to print payload", dangerous)
	}
	return payload, nil
}

func findDangerousMonitoringField(value interface{}, path string) string {
	switch typed := value.(type) {
	case map[string]interface{}:
		for key, child := range typed {
			childPath := key
			if path != "" {
				childPath = path + "." + key
			}
			lower := strings.ToLower(key)
			for _, marker := range []string{"cmdline", "command_line", "connection_string", "password", "secret", "token", "credential", "private_key"} {
				if strings.Contains(lower, marker) {
					return childPath
				}
			}
			if found := findDangerousMonitoringField(child, childPath); found != "" {
				return found
			}
		}
	case []interface{}:
		for index, child := range typed {
			if found := findDangerousMonitoringField(child, fmt.Sprintf("%s[%d]", path, index)); found != "" {
				return found
			}
		}
	}
	return ""
}

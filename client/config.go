package client

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ConfigAPIError describes a failed request to Daptin's dedicated _config API.
// Response bodies are intentionally excluded because they may be HTML or
// contain sensitive operator data.
type ConfigAPIError struct {
	Operation  string `json:"operation"`
	ConfigType string `json:"type"`
	Key        string `json:"key"`
	StatusCode int    `json:"status,omitempty"`
	Message    string `json:"message"`
}

func (e *ConfigAPIError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("config %s %s/%s failed (HTTP %d): %s", e.Operation, e.ConfigType, e.Key, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("config %s %s/%s failed: %s", e.Operation, e.ConfigType, e.Key, e.Message)
}

func (e *ExtendedClient) GetConfigValue(configType, key string) (string, error) {
	u := configValueURL(e.Endpoint, configType, key)
	resp, err := e.nextRequest().SetDebug(false).Get(u)
	if err != nil {
		return "", configRequestError("get", configType, key, 0, err.Error())
	}
	if resp.IsError() {
		return "", configRequestError("get", configType, key, resp.StatusCode(), http.StatusText(resp.StatusCode()))
	}
	if isHTMLConfigResponse(resp.Header().Get("Content-Type"), resp.Body()) {
		return "", configRequestError("get", configType, key, resp.StatusCode(), "unexpected HTML response")
	}
	return string(resp.Body()), nil
}

func (e *ExtendedClient) SetConfigValue(configType, key, value string) error {
	u := configValueURL(e.Endpoint, configType, key)
	resp, err := e.nextRequest().
		SetDebug(false).
		SetHeader("Content-Type", "text/plain; charset=utf-8").
		SetBody(value).
		Post(u)
	if err != nil {
		return configRequestError("set", configType, key, 0, err.Error())
	}
	if resp.IsError() {
		return configRequestError("set", configType, key, resp.StatusCode(), http.StatusText(resp.StatusCode()))
	}
	if isHTMLConfigResponse(resp.Header().Get("Content-Type"), resp.Body()) {
		return configRequestError("set", configType, key, resp.StatusCode(), "unexpected HTML response")
	}
	return nil
}

func isHTMLConfigResponse(contentType string, body []byte) bool {
	if strings.Contains(strings.ToLower(contentType), "text/html") {
		return true
	}
	trimmed := bytes.TrimSpace(body)
	return bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<html")) || bytes.HasPrefix(bytes.ToLower(trimmed), []byte("<!doctype html"))
}

func configValueURL(endpoint, configType, key string) string {
	return fmt.Sprintf("%s/_config/%s/%s", endpoint, url.PathEscape(configType), url.PathEscape(key))
}

func configRequestError(operation, configType, key string, status int, message string) error {
	if message == "" {
		message = "request failed"
	}
	return &ConfigAPIError{
		Operation:  operation,
		ConfigType: configType,
		Key:        key,
		StatusCode: status,
		Message:    message,
	}
}

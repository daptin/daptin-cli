package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const maxLLMErrorBytes int64 = 1 << 20

// LLMResponse exposes a successful gateway response without decoding or
// re-encoding its body. The caller must close Body.
type LLMResponse struct {
	StatusCode int
	Header     http.Header
	Body       io.ReadCloser
}

// LLMError is the normalized public error returned by Daptin's LLM gateway.
type LLMError struct {
	StatusCode int
	Code       string
	Type       string
	Message    string
	RequestID  string
	RetryAfter string
}

func (e *LLMError) Error() string {
	if e == nil {
		return ""
	}
	detail := strings.TrimSpace(e.Message)
	if detail == "" {
		detail = http.StatusText(e.StatusCode)
	}
	if e.Code != "" {
		detail = e.Code + ": " + detail
	}
	if e.RequestID != "" {
		detail += " (request_id " + e.RequestID + ")"
	}
	if e.StatusCode > 0 {
		return fmt.Sprintf("HTTP %d: %s", e.StatusCode, detail)
	}
	return detail
}

type llmErrorEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Type      string `json:"type"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

// ParseLLMError decodes the gateway's normalized error envelope. It also maps
// in-stream error codes back to their HTTP semantics because SSE headers have
// already been committed as HTTP 200 by the time those errors are emitted.
func ParseLLMError(data []byte, statusCode int, retryAfter string) *LLMError {
	var envelope llmErrorEnvelope
	_ = json.Unmarshal(data, &envelope)
	if statusCode < 400 {
		statusCode = LLMStatusForCode(envelope.Error.Code)
	}
	return &LLMError{
		StatusCode: statusCode,
		Code:       envelope.Error.Code,
		Type:       envelope.Error.Type,
		Message:    envelope.Error.Message,
		RequestID:  envelope.Error.RequestID,
		RetryAfter: retryAfter,
	}
}

func LLMStatusForCode(code string) int {
	switch code {
	case "invalid_request":
		return http.StatusBadRequest
	case "authentication_error":
		return http.StatusUnauthorized
	case "insufficient_quota":
		return http.StatusPaymentRequired
	case "permission_error":
		return http.StatusForbidden
	case "model_not_found":
		return http.StatusNotFound
	case "rate_limit_error":
		return http.StatusTooManyRequests
	case "provider_error":
		return http.StatusBadGateway
	case "service_unavailable":
		return http.StatusServiceUnavailable
	case "timeout_error":
		return http.StatusGatewayTimeout
	case "internal_error":
		return http.StatusInternalServerError
	default:
		return 0
	}
}

func (e *ExtendedClient) LLMModels(ctx context.Context) (*LLMResponse, error) {
	return e.doLLM(ctx, http.MethodGet, "/v1/models", nil)
}

func (e *ExtendedClient) LLMPost(ctx context.Context, route string, body []byte) (*LLMResponse, error) {
	switch route {
	case "/v1/chat/completions", "/v1/responses", "/v1/embeddings":
	default:
		return nil, fmt.Errorf("unsupported LLM gateway route %q", route)
	}
	return e.doLLM(ctx, http.MethodPost, route, bytes.NewReader(body))
}

func (e *ExtendedClient) doLLM(ctx context.Context, method, route string, body io.Reader) (*LLMResponse, error) {
	request, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(e.Endpoint, "/")+route, body)
	if err != nil {
		return nil, fmt.Errorf("create LLM request: %w", err)
	}
	request.Header.Set("Accept", "application/json, text/event-stream")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if e.AuthToken != "" {
		request.Header.Set("Authorization", "Bearer "+e.AuthToken)
	}

	// Use Resty's configured underlying transport without its debug middleware.
	// Debug dumps can expose bearer tokens, prompts, and generated content.
	response, err := e.HTTP.GetClient().Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode >= 400 {
		defer response.Body.Close()
		data, readErr := io.ReadAll(io.LimitReader(response.Body, maxLLMErrorBytes+1))
		if readErr != nil {
			return nil, fmt.Errorf("read LLM error response: %w", readErr)
		}
		if int64(len(data)) > maxLLMErrorBytes {
			return nil, fmt.Errorf("HTTP %d: LLM error response exceeds %d-byte limit", response.StatusCode, maxLLMErrorBytes)
		}
		parsed := ParseLLMError(data, response.StatusCode, response.Header.Get("Retry-After"))
		if parsed.Message == "" {
			parsed.Message = strings.TrimSpace(string(data))
		}
		return nil, parsed
	}
	return &LLMResponse{StatusCode: response.StatusCode, Header: response.Header.Clone(), Body: response.Body}, nil
}

package client

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// --- Pure parsing functions (values in, values out, no IO) ---

// ParseSingleResponse parses a JSON:API single-object response body.
// Returns the "data" object or an error.
func ParseSingleResponse(body []byte) (map[string]interface{}, error) {
	var envelope map[string]interface{}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	data, ok := envelope["data"].(map[string]interface{})
	if !ok || data == nil {
		return nil, ErrNotFound
	}
	return data, nil
}

// ParseListResponse parses a JSON:API list response body.
// Returns the "data" array or an error.
func ParseListResponse(body []byte) ([]map[string]interface{}, error) {
	var envelope map[string]interface{}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	dataRaw, ok := envelope["data"].([]interface{})
	if !ok {
		return nil, nil
	}

	result := make([]map[string]interface{}, 0, len(dataRaw))
	for _, item := range dataRaw {
		if m, ok := item.(map[string]interface{}); ok {
			result = append(result, m)
		}
	}
	return result, nil
}

// ParseActionResponses parses action execution response body.
func ParseActionResponses(body []byte) ([]ActionResponse, error) {
	var responses []ActionResponse
	if err := json.Unmarshal(body, &responses); err != nil {
		return nil, fmt.Errorf("parse action response: %w", err)
	}
	return responses, nil
}

// ActionResponse mirrors DaptinActionResponse but is owned by this package.
type ActionResponse struct {
	ResponseType string                 `json:"ResponseType"`
	Attributes   map[string]interface{} `json:"Attributes"`
}

// CheckStatusCode maps HTTP status codes to sentinel errors.
// Pure function.
func CheckStatusCode(code int, body string) error {
	switch {
	case code == 401:
		return fmt.Errorf("%w: %s", ErrUnauthorized, body)
	case code == 403:
		return fmt.Errorf("%w: %s", ErrForbidden, body)
	case code == 404:
		return fmt.Errorf("%w: %s", ErrNotFound, body)
	case code >= 400:
		return fmt.Errorf("HTTP %d: %s", code, body)
	}
	return nil
}

// BuildFindOneURL constructs the URL for a FindOne request.
// Pure function.
func BuildFindOneURL(endpoint, tableName, referenceId string, parameters map[string]interface{}) string {
	u := endpoint + "/api/" + tableName + "/" + referenceId
	if len(parameters) > 0 {
		params := url.Values{}
		for k, v := range parameters {
			params.Set(k, fmt.Sprintf("%v", v))
		}
		u = u + "?" + params.Encode()
	}
	return u
}

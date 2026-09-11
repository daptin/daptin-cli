package cmd

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/daptin/daptin-cli/config"
)

func TestReadActionPayloadSources(t *testing.T) {
	payload := `{"name":"checkout","options":{"quantity":2},"items":["one","two"]}`
	path := t.TempDir() + "/request.json"
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatalf("write payload: %v", err)
	}

	tests := []struct {
		name    string
		args    []string
		options actionInputOptions
		stdin   string
	}{
		{name: "positional key values", args: []string{"name=positional"}},
		{name: "positional JSON", args: []string{payload}},
		{name: "JSON flag", options: actionInputOptions{inputJSON: payload, inputJSONSet: true}},
		{name: "file", options: actionInputOptions{inputFile: path, inputFileSet: true}},
		{name: "stdin", options: actionInputOptions{inputStdin: true}, stdin: payload},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readActionPayload(tt.args, tt.options, strings.NewReader(tt.stdin))
			if err != nil {
				t.Fatalf("read payload: %v", err)
			}
			if got["name"] == nil {
				t.Fatalf("name was not decoded: %#v", got)
			}
			if tt.name != "positional key values" {
				options, ok := got["options"].(map[string]interface{})
				if !ok || options["quantity"] != float64(2) {
					t.Fatalf("nested JSON was not preserved: %#v", got)
				}
			}
		})
	}
}

func TestReadActionPayloadRejectsAmbiguousSources(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		options actionInputOptions
	}{
		{name: "positional and JSON", args: []string{"key=value"}, options: actionInputOptions{inputJSON: `{}`, inputJSONSet: true}},
		{name: "positional and file", args: []string{"key=value"}, options: actionInputOptions{inputFile: "request.json", inputFileSet: true}},
		{name: "positional and stdin", args: []string{"key=value"}, options: actionInputOptions{inputStdin: true}},
		{name: "JSON and file", options: actionInputOptions{inputJSON: `{}`, inputJSONSet: true, inputFile: "request.json", inputFileSet: true}},
		{name: "JSON and stdin", options: actionInputOptions{inputJSON: `{}`, inputJSONSet: true, inputStdin: true}},
		{name: "file and stdin", options: actionInputOptions{inputFile: "request.json", inputFileSet: true, inputStdin: true}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readActionPayload(tt.args, tt.options, strings.NewReader(`{}`))
			if err == nil || !strings.Contains(err.Error(), "provide only one") {
				t.Fatalf("expected ambiguous source error, got %v", err)
			}
		})
	}
}

func TestReadActionPayloadValidation(t *testing.T) {
	tests := []struct {
		name    string
		options actionInputOptions
		stdin   string
		want    string
	}{
		{name: "explicit empty JSON", options: actionInputOptions{inputJSONSet: true}, want: "invalid JSON payload"},
		{name: "malformed JSON", options: actionInputOptions{inputJSON: `{"secret":"do-not-print"`, inputJSONSet: true}, want: "invalid JSON payload"},
		{name: "non-object JSON", options: actionInputOptions{inputJSON: `[1,2]`, inputJSONSet: true}, want: "must be an object"},
		{name: "empty file name", options: actionInputOptions{inputFileSet: true}, want: "open input file"},
		{name: "stdin and interactive", options: actionInputOptions{inputStdin: true, interactive: true}, stdin: `{}`, want: "cannot be combined"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := readActionPayload(nil, tt.options, strings.NewReader(tt.stdin))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected error containing %q, got %v", tt.want, err)
			}
			if strings.Contains(err.Error(), "do-not-print") {
				t.Fatalf("error exposed payload contents: %v", err)
			}
		})
	}

	_, err := readActionPayload(nil, actionInputOptions{
		inputJSON:    strings.Repeat("x", int(maxJSONPayloadBytes)+1),
		inputJSONSet: true,
	}, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized payload error, got %v", err)
	}
}

func TestExecuteInputJSONReturnsScriptableCheckoutResult(t *testing.T) {
	requests := 0
	var received map[string]interface{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.URL.Path != "/action/checkout_attempt/begin_checkout" {
			t.Fatalf("unexpected request path: %s", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"ResponseType":"client.return","Attributes":{"checkout_attempt_id":"attempt-1","provider_session_id":"session-1","checkout_url":"https://checkout.example/session-1","status":"provider_created"}}]`))
	}))
	defer server.Close()

	var runErr error
	stdout := captureCRUDStdout(t, func() {
		runErr = NewApp(&config.Config{}, "test").Run([]string{
			"daptin", "--output", "json", "--endpoint", server.URL,
			"execute", "--input-json", `{"metadata":{"source":"cli"}}`, "--reference-id", "attempt-1",
			"checkout_attempt", "begin_checkout",
		})
	})
	if runErr != nil {
		t.Fatalf("run execute: %v", runErr)
	}
	if requests != 1 {
		t.Fatalf("expected one action request, got %d", requests)
	}
	attributes, ok := received["Attributes"].(map[string]interface{})
	if !ok || attributes["checkout_attempt_id"] != "attempt-1" {
		t.Fatalf("reference id was not injected: %#v", received)
	}
	metadata, ok := attributes["metadata"].(map[string]interface{})
	if !ok || metadata["source"] != "cli" {
		t.Fatalf("JSON input was not preserved: %#v", received)
	}

	var output map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("execute output is not one JSON object: %v; output: %q", err, stdout)
	}
	if output["checkout_url"] != "https://checkout.example/session-1" || output["status"] != "provider_created" {
		t.Fatalf("checkout output is not scriptable: %#v", output)
	}
}

func TestExecuteRejectsInvalidInputBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()

	err := NewApp(&config.Config{}, "test").Run([]string{
		"daptin", "--endpoint", server.URL, "execute", "--input-json", `{"invalid"`, "world", "my_action",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid JSON payload") {
		t.Fatalf("expected invalid JSON error, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("validation failure made %d HTTP requests", requests)
	}

	err = NewApp(&config.Config{}, "test").Run([]string{
		"daptin", "--endpoint", server.URL, "execute", "--input-json", "", "world", "my_action",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid JSON payload") {
		t.Fatalf("expected explicit empty JSON to fail, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("empty JSON validation made %d HTTP requests", requests)
	}
}

func TestExecutePermissionFailureDoesNotFallback(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"status":"403","title":"forbidden"}]}`))
	}))
	defer server.Close()

	err := NewApp(&config.Config{}, "test").Run([]string{
		"daptin", "--quiet", "--endpoint", server.URL, "execute", "--reference-id", "attempt-1", "checkout_attempt", "refresh_checkout",
	})
	if err == nil {
		t.Fatal("expected permission failure")
	}
	if requests != 1 {
		t.Fatalf("permission failure made %d requests; expected no fallback", requests)
	}
}

package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/config"
)

func TestBuildLLMShorthandRequests(t *testing.T) {
	tests := []struct{ operation, input, field string }{
		{"chat", "hello", "messages"},
		{"responses", "hello", "input"},
		{"embeddings", "hello", "input"},
	}
	for _, test := range tests {
		body, streaming, err := buildLLMRequest(test.operation, llmRequestOptions{model: "assistant", shorthand: test.input}, strings.NewReader(""))
		if err != nil || streaming {
			t.Fatalf("%s: streaming=%v err=%v", test.operation, streaming, err)
		}
		var value map[string]json.RawMessage
		if err := json.Unmarshal(body, &value); err != nil {
			t.Fatal(err)
		}
		if string(value["model"]) != `"assistant"` || len(value[test.field]) == 0 {
			t.Fatalf("%s request = %s", test.operation, body)
		}
	}
}

func TestBuildLLMCompleteRequestModelOverlayAndStreaming(t *testing.T) {
	body, streaming, err := buildLLMRequest("chat", llmRequestOptions{
		model: "assistant", inputJSON: `{"messages":[{"role":"user","content":"hello"}],"stream":true}`,
	}, strings.NewReader(""))
	if err != nil || !streaming || !strings.Contains(string(body), `"model":"assistant"`) {
		t.Fatalf("body=%s streaming=%v err=%v", body, streaming, err)
	}

	_, _, err = buildLLMRequest("chat", llmRequestOptions{model: "other", inputJSON: `{"model":"assistant","messages":[]}`}, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("model conflict error = %v", err)
	}
	_, _, err = buildLLMRequest("chat", llmRequestOptions{inputJSON: `{"model":"assistant","stream":false}`, stream: true}, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "stream=false") {
		t.Fatalf("stream conflict error = %v", err)
	}
}

func TestBuildLLMRequestSourceValidationAndBound(t *testing.T) {
	_, _, err := buildLLMRequest("responses", llmRequestOptions{
		model: "assistant", shorthand: "hello", inputJSON: `{"input":"other"}`,
	}, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Fatalf("source error = %v", err)
	}
	_, err = decodeBoundedLLMRequest(strings.NewReader(`[]`))
	if err == nil || !strings.Contains(err.Error(), "must be an object") {
		t.Fatalf("array error = %v", err)
	}
	_, err = decodeBoundedLLMRequest(strings.NewReader(strings.Repeat("x", int(maxLLMRequestBytes)+1)))
	if err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("limit error = %v", err)
	}
}

func TestLLMJSONOutputPreservesGatewayBodyAndAuth(t *testing.T) {
	want := `{"id":"chat_1", "choices":[{"index":0,"message":{"content":"pong"}}]}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer daptin-token" {
			t.Errorf("request path=%s auth=%q", r.URL.Path, r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, want)
	}))
	defer server.Close()

	cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "daptin-token"}}}
	var runErr error
	stdout, stderr := captureStdoutStderr(t, func() {
		runErr = NewApp(cfg, "test").Run([]string{"daptin", "--quiet", "--output", "json", "llm", "chat", "--model", "assistant", "--prompt", "hello"})
	})
	if runErr != nil {
		t.Fatalf("run: %v, stderr=%q", runErr, stderr)
	}
	if stdout != want {
		t.Fatalf("raw output changed: got %q want %q", stdout, want)
	}
}

func TestLLMRejectsSuccessfulNonGatewayContentType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = io.WriteString(w, "<html>proxy login</html>")
	}))
	defer server.Close()
	cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "token"}}}
	err := NewApp(cfg, "test").Run([]string{"daptin", "--quiet", "--output", "json", "llm", "chat", "--model", "assistant", "--prompt", "hello"})
	if err == nil || !strings.Contains(err.Error(), "expected application/json") {
		t.Fatalf("error = %v", err)
	}
}

func TestLLMStreamingGatewayErrorsAreNonZeroDespiteHTTP200(t *testing.T) {
	for _, test := range []struct {
		code    string
		message string
		status  int
	}{
		{"insufficient_quota", "insufficient quota", 402},
		{"rate_limit_error", "deployment request limit exceeded", 429},
	} {
		t.Run(test.code, func(t *testing.T) {
			errorFrame := `{"error":{"code":"` + test.code + `","type":"` + test.code + `","message":"` + test.message + `","request_id":"req_test"}}`
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: "+errorFrame+"\n\ndata: [DONE]\n\n")
			}))
			defer server.Close()
			cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "token"}}}
			var runErr error
			stdout, _ := captureStdoutStderr(t, func() {
				runErr = NewApp(cfg, "test").Run([]string{"daptin", "--quiet", "--output", "json", "llm", "chat", "--model", "assistant", "--prompt", "hello", "--stream"})
			})
			var gatewayErr *client.LLMError
			if !errors.As(runErr, &gatewayErr) || gatewayErr.StatusCode != test.status {
				t.Fatalf("error = %#v", runErr)
			}
			if stdout != errorFrame+"\n" {
				t.Fatalf("stdout = %q", stdout)
			}
		})
	}
}

func TestLLMChatStreamPrintsPrimaryDeltasThroughDone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, ": keep-alive\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"po\"}}]}\n\ndata: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ng\"}}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "token"}}}
	var runErr error
	stdout := captureCRUDStdout(t, func() {
		runErr = NewApp(cfg, "test").Run([]string{"daptin", "--quiet", "llm", "chat", "--model", "assistant", "--prompt", "hello", "--stream"})
	})
	if runErr != nil || stdout != "pong\n" {
		t.Fatalf("stdout=%q err=%v", stdout, runErr)
	}
}

func TestLLMResponsesStreamEmitsJSONLinesAndCompletedTerminal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"pong\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
	}))
	defer server.Close()
	cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "token"}}}
	var runErr error
	stdout := captureCRUDStdout(t, func() {
		runErr = NewApp(cfg, "test").Run([]string{"daptin", "--quiet", "--output", "json", "llm", "responses", "--model", "assistant", "--input", "hello", "--stream"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "output_text.delta") || !strings.Contains(lines[1], "response.completed") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestLLMResponsesStreamProviderError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: error\ndata: {\"error\":{\"code\":\"provider_error\",\"type\":\"provider_error\",\"message\":\"provider failed\",\"request_id\":\"req_provider\"}}\n\n")
	}))
	defer server.Close()
	cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "token"}}}
	var runErr error
	captureCRUDStdout(t, func() {
		runErr = NewApp(cfg, "test").Run([]string{"daptin", "--quiet", "llm", "responses", "--model", "assistant", "--input", "hello", "--stream"})
	})
	var gatewayErr *client.LLMError
	if !errors.As(runErr, &gatewayErr) || gatewayErr.StatusCode != 502 || gatewayErr.Code != "provider_error" {
		t.Fatalf("error = %#v", runErr)
	}
}

func TestLLMResponsesHumanOutputIgnoresReasoningAndWarnsWhenIncomplete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status":"incomplete","output":[{"type":"reasoning","content":[{"type":"reasoning_text","text":"private reasoning"}]},{"type":"message","content":[{"type":"output_text","text":"pong"}]}]}`)
	}))
	defer server.Close()
	cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "token"}}}
	var runErr error
	stdout, stderr := captureStdoutStderr(t, func() {
		runErr = NewApp(cfg, "test").Run([]string{"daptin", "--quiet", "llm", "responses", "--model", "assistant", "--input", "hello"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if stdout != "pong\n" || strings.Contains(stdout, "private reasoning") || !strings.Contains(stderr, "status completed") {
		t.Fatalf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestLLMEmbeddingsHumanOutputSummarizesVector(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"object":"list","model":"embedding-model","data":[{"object":"embedding","index":0,"embedding":[0.1,0.2,0.3]}]}`)
	}))
	defer server.Close()
	cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "token"}}}
	var runErr error
	stdout := captureCRUDStdout(t, func() {
		runErr = NewApp(cfg, "test").Run([]string{"daptin", "--quiet", "llm", "embeddings", "--model", "embedding-model", "--input", "hello"})
	})
	if runErr != nil {
		t.Fatal(runErr)
	}
	if !strings.Contains(stdout, "dimensions") || !strings.Contains(stdout, "3") || strings.Contains(stdout, "0.1") {
		t.Fatalf("stdout = %q", stdout)
	}
}

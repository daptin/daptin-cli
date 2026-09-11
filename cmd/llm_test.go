package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/config"
	"github.com/urfave/cli/v2"
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
		model: "assistant", inputJSON: `{"messages":[{"role":"user","content":"hello"}],"stream":true}`, inputJSONSet: true,
	}, strings.NewReader(""))
	if err != nil || !streaming || !strings.Contains(string(body), `"model":"assistant"`) {
		t.Fatalf("body=%s streaming=%v err=%v", body, streaming, err)
	}

	_, _, err = buildLLMRequest("chat", llmRequestOptions{model: "other", inputJSON: `{"model":"assistant","messages":[]}`, inputJSONSet: true}, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Fatalf("model conflict error = %v", err)
	}
	_, _, err = buildLLMRequest("chat", llmRequestOptions{inputJSON: `{"model":"assistant","stream":false}`, inputJSONSet: true, stream: true}, strings.NewReader(""))
	if err == nil || !strings.Contains(err.Error(), "stream=false") {
		t.Fatalf("stream conflict error = %v", err)
	}
}

func TestBuildLLMRequestSourceValidationAndBound(t *testing.T) {
	_, _, err := buildLLMRequest("responses", llmRequestOptions{
		model: "assistant", shorthand: "hello", inputJSON: `{"input":"other"}`, inputJSONSet: true,
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

func TestBuildLLMRequestRejectsAllAmbiguousAndEmptySources(t *testing.T) {
	tests := []struct {
		name    string
		options llmRequestOptions
		want    string
	}{
		{name: "JSON and file", options: llmRequestOptions{inputJSON: `{}`, inputJSONSet: true, inputFile: "request.json", inputFileSet: true}, want: "provide only one"},
		{name: "JSON and stdin", options: llmRequestOptions{inputJSON: `{}`, inputJSONSet: true, inputStdin: true}, want: "provide only one"},
		{name: "file and stdin", options: llmRequestOptions{inputFile: "request.json", inputFileSet: true, inputStdin: true}, want: "provide only one"},
		{name: "file and shorthand", options: llmRequestOptions{inputFile: "request.json", inputFileSet: true, shorthand: "hello"}, want: "cannot be combined"},
		{name: "stdin and shorthand", options: llmRequestOptions{inputStdin: true, shorthand: "hello"}, want: "cannot be combined"},
		{name: "empty JSON", options: llmRequestOptions{inputJSONSet: true}, want: "invalid JSON request object"},
		{name: "empty file name", options: llmRequestOptions{inputFileSet: true}, want: "open input file"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := buildLLMRequest("chat", test.options, strings.NewReader(`{}`))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("expected error containing %q, got %v", test.want, err)
			}
		})
	}
}

func TestLLMCompleteRequestFileAndStdin(t *testing.T) {
	payload := `{"model":"assistant","messages":[{"role":"user","content":"hello"}]}`
	path := t.TempDir() + "/request.json"
	if err := os.WriteFile(path, []byte(payload), 0600); err != nil {
		t.Fatalf("write request: %v", err)
	}

	tests := []struct {
		name         string
		commandArgs  []string
		stdinPayload string
	}{
		{name: "file", commandArgs: []string{"--input-file", path}},
		{name: "stdin", commandArgs: []string{"--input-stdin"}, stdinPayload: payload},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var received map[string]interface{}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
					t.Fatalf("decode request: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"choices":[{"index":0,"message":{"content":"pong"}}]}`)
			}))
			defer server.Close()

			oldStdin := os.Stdin
			if test.stdinPayload != "" {
				reader, writer, err := os.Pipe()
				if err != nil {
					t.Fatal(err)
				}
				if _, err := writer.WriteString(test.stdinPayload); err != nil {
					t.Fatal(err)
				}
				_ = writer.Close()
				os.Stdin = reader
				defer func() {
					os.Stdin = oldStdin
					_ = reader.Close()
				}()
			}

			args := []string{"daptin", "--quiet", "--endpoint", server.URL, "llm", "chat"}
			args = append(args, test.commandArgs...)
			var runErr error
			stdout := captureCRUDStdout(t, func() {
				runErr = NewApp(&config.Config{}, "test").Run(args)
			})
			if runErr != nil || stdout != "pong\n" {
				t.Fatalf("stdout=%q error=%v", stdout, runErr)
			}
			if received["model"] != "assistant" {
				t.Fatalf("request body was not preserved: %#v", received)
			}
		})
	}
}

func TestLLMExplicitEmptyJSONFailsBeforeRequest(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
	}))
	defer server.Close()

	err := NewApp(&config.Config{}, "test").Run([]string{
		"daptin", "--quiet", "--endpoint", server.URL, "llm", "chat",
		"--model", "assistant", "--input-json", "",
	})
	if err == nil || !strings.Contains(err.Error(), "invalid JSON request object") {
		t.Fatalf("expected explicit empty JSON error, got %v", err)
	}
	if requests != 0 {
		t.Fatalf("invalid input made %d HTTP requests", requests)
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
		_, _ = io.WriteString(w, "event: response.output_text.delta\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\ndata: \"pong\"}\n\nevent: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"status\":\"completed\"}}\n\n")
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
	if len(lines) != 2 || !strings.Contains(lines[0], `"delta":"pong"`) || !strings.Contains(lines[1], "response.completed") {
		t.Fatalf("stdout = %q", stdout)
	}
}

func TestLLMCommandCancellationReturnsExit130(t *testing.T) {
	started := make(chan struct{})
	cancelled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		close(started)
		<-r.Context().Done()
		close(cancelled)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	app := NewApp(&config.Config{}, "test")
	app.ExitErrHandler = func(_ *cli.Context, _ error) {}
	go func() {
		done <- app.RunContext(ctx, []string{
			"daptin", "--quiet", "--endpoint", server.URL, "llm", "chat",
			"--model", "assistant", "--prompt", "hello", "--stream",
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("stream request did not start")
	}
	cancel()

	var err error
	select {
	case err = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelled command did not return")
	}
	exitErr, ok := err.(interface{ ExitCode() int })
	if !ok || exitErr.ExitCode() != 130 {
		t.Fatalf("cancellation error = %#v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("cancellation did not reach server")
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

package client

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLLMModelsUsesDaptinEndpointAndBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/models" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer daptin-token" {
			t.Fatalf("authorization = %q", got)
		}
		_, _ = io.WriteString(w, `{"object":"list","data":[]}`)
	}))
	defer server.Close()

	response, err := New(server.URL+"/", "daptin-token", true).LLMModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	if string(body) != `{"object":"list","data":[]}` {
		t.Fatalf("body changed: %s", body)
	}
}

func TestLLMPostRejectsArbitraryRoute(t *testing.T) {
	_, err := New("http://example.test", "", false).LLMPost(context.Background(), "/v1/provider-passthrough", nil)
	if err == nil || !strings.Contains(err.Error(), "unsupported LLM gateway route") {
		t.Fatalf("error = %v", err)
	}
}

func TestLLMHTTPErrorPreservesNormalizedDetails(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "9")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"code":"rate_limit_error","type":"rate_limit_error","message":"deployment request limit exceeded","request_id":"req_live"}}`)
	}))
	defer server.Close()

	_, err := New(server.URL, "token", false).LLMPost(context.Background(), "/v1/embeddings", []byte(`{}`))
	var gatewayErr *LLMError
	if !errors.As(err, &gatewayErr) {
		t.Fatalf("error type = %T: %v", err, err)
	}
	if gatewayErr.StatusCode != 429 || gatewayErr.Code != "rate_limit_error" || gatewayErr.RequestID != "req_live" || gatewayErr.RetryAfter != "9" {
		t.Fatalf("error = %#v", gatewayErr)
	}
}

func TestLLMRequestCancellationReachesServer(t *testing.T) {
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
	go func() {
		response, err := New(server.URL, "token", false).LLMPost(ctx, "/v1/chat/completions", []byte(`{}`))
		if err == nil {
			defer response.Body.Close()
			_, err = io.Copy(io.Discard, response.Body)
		}
		done <- err
	}()
	<-started
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v", err)
	}
	<-cancelled
}

func TestParseStreamingErrorStatus(t *testing.T) {
	tests := []struct {
		code string
		want int
	}{{"insufficient_quota", 402}, {"rate_limit_error", 429}, {"provider_error", 502}}
	for _, test := range tests {
		data := []byte(`{"error":{"code":"` + test.code + `","message":"failed"}}`)
		if got := ParseLLMError(data, 0, "").StatusCode; got != test.want {
			t.Errorf("%s status = %d, want %d", test.code, got, test.want)
		}
	}
}

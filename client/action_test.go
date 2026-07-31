package client

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExecuteRejectsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"status":"403"}]}`))
	}))
	defer server.Close()

	_, err := New(server.URL, "", false).Execute("signin", "user_account", map[string]interface{}{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
}

func TestExecuteParsesSuccessfulActionResponses(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"ResponseType":"client.notify","Attributes":{"type":"success","message":"ok"}}]`))
	}))
	defer server.Close()

	responses, err := New(server.URL, "", false).Execute("probe", "world", map[string]interface{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 || responses[0].ResponseType != "client.notify" {
		t.Fatalf("unexpected responses: %#v", responses)
	}
}

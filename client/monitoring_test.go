package client

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthReturnsMinimalReadiness(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/world" || r.URL.Query().Get("page[size]") != "1" {
			t.Fatalf("unexpected health request: %s", r.URL.String())
		}
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer server.Close()

	result, err := New(server.URL, "", false).Health()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["healthy"] != true || result["status_code"] != 200 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestStatisticsRequiresAuthentication(t *testing.T) {
	_, err := New("http://example.test", "", false).Statistics()
	if err == nil || !strings.Contains(err.Error(), "authenticated context") {
		t.Fatalf("expected authentication error, got %v", err)
	}
}

func TestStatisticsRejectsUnsafePayloadWithoutReturningIt(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"process":{"top_processes":[{"cmdline":"daptin -db_connection_string secret"}]}}`))
	}))
	defer server.Close()

	payload, err := New(server.URL, "admin-token", false).Statistics()
	if err == nil || !strings.Contains(err.Error(), "process.top_processes[0].cmdline") {
		t.Fatalf("expected unsafe payload error, got %v", err)
	}
	if payload != nil || strings.Contains(err.Error(), "db_connection_string") {
		t.Fatalf("unsafe payload leaked: payload=%#v err=%v", payload, err)
	}
}

func TestStatisticsReturnsSafePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer admin-token" {
			t.Fatalf("missing authorization header")
		}
		_, _ = w.Write([]byte(`{"cpu":{"cores":8},"process":{"top_processes":[{"pid":1,"name":"daptin"}]}}`))
	}))
	defer server.Close()

	payload, err := New(server.URL, "admin-token", false).Statistics()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if payload["cpu"] == nil {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

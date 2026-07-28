package client

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetConfigValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/_config/backend/mail.default_server_hostname" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer admin-token" {
			t.Fatalf("missing auth header: %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte("mail.canaster.in"))
	}))
	defer server.Close()

	value, err := New(server.URL, "admin-token", false).GetConfigValue("backend", "mail.default_server_hostname")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != "mail.canaster.in" {
		t.Fatalf("unexpected value: %q", value)
	}
}

func TestSetConfigValueSendsPlainTextAndAcceptsEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/_config/backend/enable_https" {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "text/plain") {
			t.Fatalf("unexpected content type: %q", r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != "true" {
			t.Fatalf("unexpected body: %q", body)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	if err := New(server.URL, "", false).SetConfigValue("backend", "enable_https", "true"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigValueHTMLFailureReturnsStructuredSafeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`<html><body>proxy details and secret-value</body></html>`))
	}))
	defer server.Close()

	_, err := New(server.URL, "", false).GetConfigValue("backend", "jwt.secret")
	if err == nil {
		t.Fatal("expected error")
	}
	var apiErr *ConfigAPIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected ConfigAPIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusForbidden || apiErr.Message != "Forbidden" {
		t.Fatalf("unexpected structured error: %#v", apiErr)
	}
	if strings.Contains(err.Error(), "<html>") || strings.Contains(err.Error(), "secret-value") {
		t.Fatalf("unsafe response body leaked: %v", err)
	}
}

func TestConfigValueSuccessfulHTMLResponseIsRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>sign in</body></html>`))
	}))
	defer server.Close()

	_, err := New(server.URL, "", false).GetConfigValue("backend", "hostname")
	if err == nil || !strings.Contains(err.Error(), "unexpected HTML response") {
		t.Fatalf("expected safe HTML response error, got %v", err)
	}
}

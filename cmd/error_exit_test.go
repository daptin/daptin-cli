package cmd

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/daptin/daptin-cli/config"
)

func TestCreateReturnsServerRejection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"errors":[{"status":"403","title":"forbidden"}]}`))
	}))
	defer server.Close()

	err := NewApp(&config.Config{}, "test").Run([]string{"daptin", "--quiet", "--endpoint", server.URL, "create", "document", "name=probe"})
	if err == nil || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("expected create rejection, got %v", err)
	}
}

func TestExecuteReturnsActionErrorNotification(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"ResponseType":"client.notify","Attributes":{"type":"error","message":"invalid credentials"}}]`))
	}))
	defer server.Close()

	err := NewApp(&config.Config{}, "test").Run([]string{"daptin", "--quiet", "--endpoint", server.URL, "execute", "user_account", "signin", "email=x", "password=y"})
	if err == nil || err.Error() != "invalid credentials" {
		t.Fatalf("expected action rejection, got %v", err)
	}
}

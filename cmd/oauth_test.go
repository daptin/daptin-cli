package cmd

import (
	"os"
	"path/filepath"
	"testing"

	daptinClient "github.com/daptin/daptin-go-client"
)

func TestOAuthClientSecretFromEnv(t *testing.T) {
	t.Setenv("DAPTIN_TEST_CLIENT_SECRET", "env-secret")
	secret, err := oauthClientSecret("", "DAPTIN_TEST_CLIENT_SECRET", "")
	if err != nil {
		t.Fatal(err)
	}
	if secret != "env-secret" {
		t.Fatalf("expected env-secret, got %q", secret)
	}
}

func TestOAuthClientSecretFromFileTrimsWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	secret, err := oauthClientSecret("", "", path)
	if err != nil {
		t.Fatal(err)
	}
	if secret != "file-secret" {
		t.Fatalf("expected file-secret, got %q", secret)
	}
}

func TestOAuthClientSecretRequiresSingleSource(t *testing.T) {
	if _, err := oauthClientSecret("direct", "ENV", ""); err == nil {
		t.Fatal("expected error for multiple secret sources")
	}
}

func TestRedirectURLFromResponses(t *testing.T) {
	responses := []daptinClient.DaptinActionResponse{
		{
			ResponseType: "client.redirect",
			Attributes: map[string]interface{}{
				"location": "https://provider.example/auth",
			},
		},
	}
	got := redirectURLFromResponses(responses)
	if got != "https://provider.example/auth" {
		t.Fatalf("unexpected redirect URL: %q", got)
	}
}

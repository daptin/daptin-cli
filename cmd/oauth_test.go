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

func TestOAuthAppRegisterAttrsDefaults(t *testing.T) {
	attrs := oauthAppRegisterAttrs("App Login", []string{"https://app.example.com/auth/daptin/callback"}, nil, nil, true)
	if attrs["name"] != "App Login" {
		t.Fatalf("unexpected name: %v", attrs["name"])
	}
	if attrs["redirect_uris"] != "https://app.example.com/auth/daptin/callback" {
		t.Fatalf("unexpected redirect_uris: %v", attrs["redirect_uris"])
	}
	if attrs["scopes"] != "openid profile email" {
		t.Fatalf("unexpected scopes: %v", attrs["scopes"])
	}
	if attrs["grants"] != "authorization_code refresh_token" {
		t.Fatalf("unexpected grants: %v", attrs["grants"])
	}
	if attrs["is_confidential"] != true {
		t.Fatalf("expected confidential client, got %v", attrs["is_confidential"])
	}
}

func TestOAuthListStringAcceptsRepeatedCommaAndSpaceValues(t *testing.T) {
	got := oauthListString([]string{"openid,profile", "email offline_access"}, "")
	if got != "openid profile email offline_access" {
		t.Fatalf("unexpected list: %q", got)
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

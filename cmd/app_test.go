package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/daptin/daptin-cli/config"
)

func TestNewApp_WarnsForExpiredSavedContextToken(t *testing.T) {
	cfg := &config.Config{
		CurrentContext: "dev",
		Hosts: []config.HostEndpoint{
			{Name: "dev", Endpoint: "http://example.test", Token: appTestJWT(`{"exp":1}`)},
		},
	}
	app := NewApp(cfg, "test")

	stderr := captureStderr(t, func() {
		err := app.Run([]string{"daptin", "--quiet", "context", "list"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stderr, `Warning: saved auth token for context "dev" expired at 1970-01-01T00:00:01Z; sign in again to refresh it.`) {
		t.Fatalf("expected expired token warning, got: %q", stderr)
	}
}

func TestNewApp_DoesNotWarnForValidSavedContextToken(t *testing.T) {
	cfg := &config.Config{
		CurrentContext: "dev",
		Hosts: []config.HostEndpoint{
			{Name: "dev", Endpoint: "http://example.test", Token: appTestJWT(`{"exp":4102444800}`)},
		},
	}
	app := NewApp(cfg, "test")

	stderr := captureStderr(t, func() {
		err := app.Run([]string{"daptin", "--quiet", "context", "list"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if strings.Contains(stderr, "saved auth token") {
		t.Fatalf("did not expect expired token warning, got: %q", stderr)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	oldStderr := os.Stderr
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stderr = writer
	defer func() {
		os.Stderr = oldStderr
	}()

	output := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, reader)
		output <- buf.String()
	}()

	fn()

	_ = writer.Close()
	os.Stderr = oldStderr
	return <-output
}

func appTestJWT(payload string) string {
	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return fmt.Sprintf("%s.%s.", encodedHeader, encodedPayload)
}

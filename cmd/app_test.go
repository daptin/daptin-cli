package cmd

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
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

func TestNewApp_DoesNotMarkExpiredSavedContextTokenAuthenticated(t *testing.T) {
	cfg := &config.Config{
		CurrentContext: "dev",
		Hosts: []config.HostEndpoint{
			{Name: "dev", Endpoint: "http://example.test", Token: appTestJWT(`{"exp":1}`)},
		},
	}
	app := NewApp(cfg, "test")

	var runErr error
	stdout, stderr := captureStdoutStderr(t, func() {
		runErr = app.Run([]string{"daptin", "context", "list"})
	})
	if runErr != nil {
		t.Fatalf("unexpected error: %v", runErr)
	}

	if strings.Contains(stderr, "authenticated") {
		t.Fatalf("expired token should not be reported authenticated on startup, got stderr: %q", stderr)
	}
	if !strings.Contains(stderr, "Using dev (http://example.test)") {
		t.Fatalf("expected unauthenticated context banner, got stderr: %q", stderr)
	}
	if strings.Contains(stdout, "authenticated") {
		t.Fatalf("expired token should not be reported authenticated in context list, got stdout: %q", stdout)
	}
	if !strings.Contains(stdout, "* dev  http://example.test (expired)") {
		t.Fatalf("expected expired context label, got stdout: %q", stdout)
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

func TestNewAppFromConfig_ExplicitConfigIsIsolated(t *testing.T) {
	for _, flag := range []string{"--config", "-c"} {
		t.Run(flag, func(t *testing.T) {
			dir := t.TempDir()
			defaultPath := filepath.Join(dir, "default.yaml")
			customPath := filepath.Join(dir, "isolated", "config.yaml")
			defaultContents := []byte("currentContext: prod\nhosts:\n- name: prod\n  endpoint: https://example.invalid\n  token: secret\n")
			if err := os.WriteFile(defaultPath, defaultContents, 0600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DAPTIN_CLI_CONFIG", defaultPath)

			run := func(args ...string) string {
				t.Helper()
				var runErr error
				stderr := captureStderr(t, func() {
					runErr = NewAppFromConfig("test").Run(args)
				})
				if runErr != nil {
					t.Fatalf("run %v: %v", args, runErr)
				}
				return stderr
			}

			run("daptin", "--quiet", flag, customPath, "context", "add", "isolated", "http://127.0.0.1:16336")
			run("daptin", "--quiet", flag, customPath, "context", "set", "isolated")
			stderr := run("daptin", flag, customPath, "context", "list")

			gotDefault, err := os.ReadFile(defaultPath)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(gotDefault, defaultContents) {
				t.Fatalf("default config was modified:\n%s", gotDefault)
			}

			isolated, err := config.Load(customPath)
			if err != nil {
				t.Fatal(err)
			}
			if isolated.CurrentContext != "isolated" || len(isolated.Hosts) != 1 {
				t.Fatalf("unexpected isolated config: %#v", isolated)
			}
			if isolated.Hosts[0].Endpoint != "http://127.0.0.1:16336" {
				t.Fatalf("unexpected isolated endpoint: %q", isolated.Hosts[0].Endpoint)
			}
			if !strings.Contains(stderr, "Using isolated (http://127.0.0.1:16336)") || strings.Contains(stderr, "prod") {
				t.Fatalf("context list did not use isolated config: %q", stderr)
			}
		})
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()

	_, stderr := captureStdoutStderr(t, fn)
	return stderr
}

func captureStdoutStderr(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter
	defer func() {
		os.Stdout = oldStdout
		os.Stderr = oldStderr
	}()

	stdoutOutput := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutReader)
		stdoutOutput <- buf.String()
	}()
	stderrOutput := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrReader)
		stderrOutput <- buf.String()
	}()

	fn()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	return <-stdoutOutput, <-stderrOutput
}

func appTestJWT(payload string) string {
	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return fmt.Sprintf("%s.%s.", encodedHeader, encodedPayload)
}

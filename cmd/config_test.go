package cmd

import (
	"bytes"
	"flag"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/daptin/daptin-cli/client"
	"github.com/daptin/daptin-cli/config"
	"github.com/urfave/cli/v2"
)

func TestConfigCommandGetAndSet(t *testing.T) {
	var setBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/_config/backend/hostname":
			_, _ = w.Write([]byte("api.canaster.in"))
		case r.Method == http.MethodGet && r.URL.Path == "/_config/backend/jwt.secret":
			_, _ = w.Write([]byte("do-not-print"))
		case r.Method == http.MethodPost && r.URL.Path == "/_config/backend/jwt.secret":
			body := make([]byte, r.ContentLength)
			_, _ = r.Body.Read(body)
			setBody = string(body)
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := &config.Config{CurrentContext: "test", Hosts: []config.HostEndpoint{{Name: "test", Endpoint: server.URL, Token: "admin-token"}}}
	run := func(args ...string) string {
		t.Helper()
		var runErr error
		stdout, _ := captureConfigStdoutStderr(t, func() {
			runErr = NewApp(cfg, "test").Run(args)
		})
		if runErr != nil {
			t.Fatalf("run %v: %v", args, runErr)
		}
		return stdout
	}

	hostOutput := run("daptin", "--quiet", "--output", "json", "config", "get", "backend", "hostname")
	if !strings.Contains(hostOutput, `"value": "api.canaster.in"`) || !strings.Contains(hostOutput, `"redacted": false`) {
		t.Fatalf("unexpected config get output: %s", hostOutput)
	}
	secretOutput := run("daptin", "--quiet", "--output", "json", "config", "get", "backend", "jwt.secret")
	if strings.Contains(secretOutput, "do-not-print") || !strings.Contains(secretOutput, `"value": "[REDACTED]"`) {
		t.Fatalf("secret was not redacted: %s", secretOutput)
	}
	shownSecretOutput := run("daptin", "--quiet", "--output", "json", "config", "get", "--show-secrets", "backend", "jwt.secret")
	if !strings.Contains(shownSecretOutput, `"value": "do-not-print"`) || !strings.Contains(shownSecretOutput, `"redacted": false`) {
		t.Fatalf("secret was not shown when explicitly requested: %s", shownSecretOutput)
	}
	setOutput := run("daptin", "--quiet", "--output", "json", "config", "set", "backend", "jwt.secret", "new-secret")
	if setBody != "new-secret" {
		t.Fatalf("unexpected set body: %q", setBody)
	}
	if strings.Contains(setOutput, "new-secret") || !strings.Contains(setOutput, `"updated": true`) {
		t.Fatalf("set output exposed value or omitted status: %s", setOutput)
	}
	dashValueOutput := run("daptin", "--quiet", "--output", "json", "config", "set", "--value", "-1", "backend", "jwt.secret")
	if setBody != "-1" {
		t.Fatalf("unexpected dash-prefixed set body: %q", setBody)
	}
	if strings.Contains(dashValueOutput, "-1") || !strings.Contains(dashValueOutput, `"updated": true`) {
		t.Fatalf("dash-prefixed set output exposed value or omitted status: %s", dashValueOutput)
	}
}

func TestConfigCommandShowSecrets(t *testing.T) {
	if !isSensitiveConfigKey("mail.password") || !isSensitiveConfigKey("jwt.secret") || isSensitiveConfigKey("hostname") {
		t.Fatal("unexpected sensitive-key classification")
	}
}

func TestConfigCommandJSONError(t *testing.T) {
	set := flag.NewFlagSet("test", flag.ContinueOnError)
	set.String("output", "json", "")
	if err := set.Parse([]string{"--output", "json"}); err != nil {
		t.Fatal(err)
	}
	c := cli.NewContext(nil, set, nil)

	var gotErr error
	_, stderr := captureConfigStdoutStderr(t, func() {
		gotErr = configCommandError(c, &client.ConfigAPIError{
			Operation: "get", ConfigType: "backend", Key: "jwt.secret", StatusCode: 403, Message: "Forbidden",
		})
	})
	if gotErr == nil {
		t.Fatal("expected exit error")
	}
	for _, expected := range []string{`"operation":"get"`, `"status":403`, `"message":"Forbidden"`} {
		if !strings.Contains(stderr, expected) {
			t.Fatalf("expected %s in JSON error: %s", expected, stderr)
		}
	}
}

func captureConfigStdoutStderr(t *testing.T, fn func()) (string, string) {
	t.Helper()

	oldStdout, oldStderr := os.Stdout, os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stderr: %v", err)
	}
	os.Stdout, os.Stderr = stdoutWriter, stderrWriter
	defer func() { os.Stdout, os.Stderr = oldStdout, oldStderr }()

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
	os.Stdout, os.Stderr = oldStdout, oldStderr
	return <-stdoutOutput, <-stderrOutput
}

func TestConfigKeyArgsRejectsPathSegments(t *testing.T) {
	for _, value := range []string{"../backend", "backend/key", "backend?x=y", ""} {
		if value != "" && configSegmentPattern.MatchString(value) {
			t.Fatalf("expected invalid segment: %q", value)
		}
	}
}

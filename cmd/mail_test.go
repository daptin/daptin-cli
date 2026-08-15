package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMailRequiresExplicitConfig(t *testing.T) {
	err := NewAppFromConfig("test").Run([]string{
		"daptin", "--quiet", "mail", "smtp-check", "--address", "127.0.0.1:1", "--tls", "implicit", "--server-name", "mail.test",
	})
	if err == nil || !strings.Contains(err.Error(), "explicit --config") {
		t.Fatalf("expected explicit config error, got %v", err)
	}
}

func TestMailRejectsEndpointOverride(t *testing.T) {
	path := writeMailTestConfig(t)
	err := NewAppFromConfig("test").Run([]string{
		"daptin", "--quiet", "--config", path, "--endpoint", "http://other.test", "mail", "smtp-check",
		"--address", "127.0.0.1:1", "--tls", "implicit", "--server-name", "mail.test",
	})
	if err == nil || !strings.Contains(err.Error(), "do not allow --endpoint") {
		t.Fatalf("expected endpoint override error, got %v", err)
	}
}

func TestMailAcceptsExplicitConfigFromEnvironment(t *testing.T) {
	path := writeMailTestConfig(t)
	t.Setenv("DAPTIN_CLI_CONFIG", path)
	err := NewAppFromConfig("test").Run([]string{
		"daptin", "--quiet", "mail", "smtp-check", "--address", "127.0.0.1:1", "--tls", "implicit", "--server-name", "mail.test",
	})
	if err == nil || err.Error() != "mail listener connection failed" {
		t.Fatalf("expected command to pass config validation, got %v", err)
	}
}

func TestSMTPDeliveryDryRunDoesNotRequireIdentities(t *testing.T) {
	path := writeMailTestConfig(t)
	err := NewAppFromConfig("test").Run([]string{
		"daptin", "--quiet", "--config", path, "mail", "smtp-delivery-check",
		"--address", "127.0.0.1:1", "--tls", "implicit", "--server-name", "mail.test", "--dry-run",
	})
	if err == nil || err.Error() != "mail listener connection failed" {
		t.Fatalf("expected identity-free dry-run to reach listener check, got %v", err)
	}
}

func TestMailQualificationDryRunDoesNotRequireIdentities(t *testing.T) {
	path := writeMailTestConfig(t)
	err := NewAppFromConfig("test").Run([]string{
		"daptin", "--quiet", "--config", path, "mail", "qualify",
		"--smtp-address", "127.0.0.1:1", "--smtp-tls", "implicit",
		"--imap-address", "127.0.0.1:2", "--imap-tls", "implicit",
		"--server-name", "mail.test", "--dry-run",
	})
	if err == nil || err.Error() != "mail listener connection failed" {
		t.Fatalf("expected identity-free dry-run to reach listener check, got %v", err)
	}
}

func writeMailTestConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := []byte("currentContext: test\nhosts:\n- name: test\n  endpoint: http://127.0.0.1:6336\n  token: \"\"\n")
	if err := os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

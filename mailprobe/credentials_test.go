package mailprobe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadCredentialSourcesAndPermissions(t *testing.T) {
	got, err := ReadCredential(strings.NewReader("secret\n"), true, "")
	if err != nil || string(got) != "secret" {
		t.Fatalf("stdin credential = %q, %v", got, err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte("file-secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = ReadCredential(nil, false, path)
	if err != nil || string(got) != "file-secret" {
		t.Fatalf("file credential = %q, %v", got, err)
	}
	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadCredential(nil, false, path); err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("expected permission rejection, got %v", err)
	}
}

func TestReadCredentialRejectsUnsafeInputs(t *testing.T) {
	tests := []string{"", "one\ntwo", "bad\x00value"}
	for _, value := range tests {
		if _, err := ReadCredential(strings.NewReader(value), true, ""); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
	if _, err := ReadCredential(strings.NewReader("secret"), true, "also-file"); err == nil {
		t.Fatal("expected multiple sources to be rejected")
	}
}

func TestReadQualificationCredentials(t *testing.T) {
	got, err := ReadQualificationCredentials(strings.NewReader(`{"smtp_password":"smtp-secret","imap_password":"imap-secret"}`), true, "")
	if err != nil {
		t.Fatal(err)
	}
	if got.SMTPPassword != "smtp-secret" || got.IMAPPassword != "imap-secret" {
		t.Fatalf("unexpected credentials: %#v", got)
	}
	trimmed, err := ReadQualificationCredentials(strings.NewReader("{\"smtp_password\":\"smtp-secret\\n\",\"imap_password\":\"imap-secret\\n\"}"), true, "")
	if err != nil || trimmed.SMTPPassword != "smtp-secret" || trimmed.IMAPPassword != "imap-secret" {
		t.Fatalf("expected trailing newline normalization, got %#v, %v", trimmed, err)
	}
	if _, err := ReadQualificationCredentials(strings.NewReader(`{"smtp_password":"x","imap_password":"y","extra":"z"}`), true, ""); err == nil {
		t.Fatal("expected unknown field rejection")
	}
}

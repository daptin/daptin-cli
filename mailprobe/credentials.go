package mailprobe

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxCredentialBytes = 4096

type QualificationCredentials struct {
	SMTPPassword string `json:"smtp_password"`
	IMAPPassword string `json:"imap_password"`
}

func ReadCredential(stdin io.Reader, useStdin bool, file string) ([]byte, error) {
	data, err := readSecretSource(stdin, useStdin, file)
	if err != nil {
		return nil, err
	}
	value, err := normalizeSecret(data)
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

func ReadQualificationCredentials(stdin io.Reader, useStdin bool, file string) (QualificationCredentials, error) {
	data, err := readSecretSource(stdin, useStdin, file)
	if err != nil {
		return QualificationCredentials{}, err
	}
	var credentials QualificationCredentials
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&credentials); err != nil {
		return QualificationCredentials{}, fmt.Errorf("credentials must be a JSON object with smtp_password and imap_password")
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return QualificationCredentials{}, fmt.Errorf("credentials contain trailing data")
	}
	smtpPassword, err := normalizeSecret([]byte(credentials.SMTPPassword))
	if err != nil {
		return QualificationCredentials{}, fmt.Errorf("SMTP credential is invalid")
	}
	imapPassword, err := normalizeSecret([]byte(credentials.IMAPPassword))
	if err != nil {
		return QualificationCredentials{}, fmt.Errorf("IMAP credential is invalid")
	}
	credentials.SMTPPassword = smtpPassword
	credentials.IMAPPassword = imapPassword
	return credentials, nil
}

func readSecretSource(stdin io.Reader, useStdin bool, file string) ([]byte, error) {
	if useStdin == (file != "") {
		return nil, fmt.Errorf("provide exactly one credential source")
	}
	var reader io.Reader = stdin
	if file != "" {
		clean := filepath.Clean(file)
		info, err := os.Lstat(clean)
		if err != nil {
			return nil, fmt.Errorf("inspect credential file: %w", err)
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("credential file must be a regular file, not a symlink")
		}
		if info.Mode().Perm() != 0600 {
			return nil, fmt.Errorf("credential file permissions must be 0600")
		}
		handle, err := os.Open(clean)
		if err != nil {
			return nil, fmt.Errorf("open credential file: %w", err)
		}
		defer handle.Close()
		reader = handle
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxCredentialBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read credential: %w", err)
	}
	if len(data) > maxCredentialBytes {
		return nil, fmt.Errorf("credential input exceeds 4096 bytes")
	}
	return data, nil
}

func normalizeSecret(data []byte) (string, error) {
	value := string(data)
	value = strings.TrimSuffix(value, "\n")
	value = strings.TrimSuffix(value, "\r")
	if value == "" || containsControl(value) {
		return "", fmt.Errorf("credential must be non-empty and single-line")
	}
	return value, nil
}

func containsControl(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}

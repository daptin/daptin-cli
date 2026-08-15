package mailprobe

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"
)

type TLSMode string

const (
	TLSImplicit TLSMode = "implicit"
	TLSStartTLS TLSMode = "starttls"
)

func ParseTLSMode(value string) (TLSMode, error) {
	mode := TLSMode(strings.ToLower(strings.TrimSpace(value)))
	if mode != TLSImplicit && mode != TLSStartTLS {
		return "", fmt.Errorf("TLS mode must be implicit or starttls")
	}
	return mode, nil
}

type Connection struct {
	Address    string
	ServerName string
	CAFile     string
	TLSMode    TLSMode
	Timeout    time.Duration
}

type TLSResult struct {
	Verified  bool
	Version   string
	ExpiresAt time.Time
}

func tlsResult(state tls.ConnectionState) TLSResult {
	result := TLSResult{Verified: state.HandshakeComplete, Version: tlsVersion(state.Version)}
	if len(state.PeerCertificates) > 0 {
		result.ExpiresAt = state.PeerCertificates[0].NotAfter.UTC()
	}
	return result
}

func tlsVersion(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS1.3"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS10:
		return "TLS1.0"
	default:
		return "unknown"
	}
}

type SMTPCheckResult struct {
	Connected           bool
	SMTPReady           bool
	StartTLSAdvertised  bool
	AuthLoginAdvertised bool
	TLS                 TLSResult
	Duration            time.Duration
}

type SMTPDeliveryRequest struct {
	Connection
	Username      string
	Password      []byte
	From          string
	To            string
	CorrelationID string
}

type SMTPDeliveryResult struct {
	SMTPCheckResult
	Authenticated     bool
	SenderAccepted    bool
	RecipientAccepted bool
	MessageAccepted   bool
}

type IMAPSearchRequest struct {
	Connection
	Username      string
	Password      []byte
	Folder        string
	CorrelationID string
	PollTimeout   time.Duration
	PollInterval  time.Duration
}

type IMAPResult struct {
	Connected      bool
	TLS            TLSResult
	Authenticated  bool
	FolderSelected bool
	MessageFound   bool
	PollAttempts   int
	Duration       time.Duration
}

type ProtocolError struct {
	Message string
}

func (e *ProtocolError) Error() string { return e.Message }

func protocolError(message string) error { return &ProtocolError{Message: message} }

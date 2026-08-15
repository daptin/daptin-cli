package mailprobe

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var correlationPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,128}$`)

func ValidateConnection(c Connection) error {
	if _, _, err := net.SplitHostPort(c.Address); err != nil {
		return fmt.Errorf("mail address must be host:port")
	}
	if strings.TrimSpace(c.ServerName) == "" || hasControl(c.ServerName) {
		return fmt.Errorf("TLS server name is required")
	}
	if c.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}
	if c.TLSMode != TLSImplicit && c.TLSMode != TLSStartTLS {
		return fmt.Errorf("TLS mode must be implicit or starttls")
	}
	return nil
}

func ValidateCorrelationID(value string) error {
	if !correlationPattern.MatchString(value) {
		return fmt.Errorf("correlation ID must be 1-128 ASCII letters, digits, dots, underscores, or hyphens")
	}
	return nil
}

func ValidateProtocolValue(name, value string, max int) error {
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if len(value) > max || hasControl(value) {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

func hasControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

package config

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"
)

func TestInspectTokenExpiry_Expired(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	status, expiresAt := InspectTokenExpiry(testJWT(`{"exp":1999}`), now)

	if status != TokenExpiryExpired {
		t.Fatalf("expected expired, got %v", status)
	}
	if !expiresAt.Equal(time.Unix(1999, 0).UTC()) {
		t.Fatalf("unexpected expiry: %s", expiresAt)
	}
}

func TestInspectTokenExpiry_Valid(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	status, expiresAt := InspectTokenExpiry(testJWT(`{"exp":2001}`), now)

	if status != TokenExpiryValid {
		t.Fatalf("expected valid, got %v", status)
	}
	if !expiresAt.Equal(time.Unix(2001, 0).UTC()) {
		t.Fatalf("unexpected expiry: %s", expiresAt)
	}
}

func TestInspectTokenExpiry_NotInspectable(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	tests := []struct {
		name  string
		token string
	}{
		{"opaque", "token"},
		{"missing exp", testJWT(`{"sub":"user"}`)},
		{"non numeric exp", testJWT(`{"exp":"soon"}`)},
		{"bad payload", "a.b.c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, expiresAt := InspectTokenExpiry(tt.token, now)
			if status != TokenExpiryNotInspectable {
				t.Fatalf("expected not inspectable, got %v", status)
			}
			if !expiresAt.IsZero() {
				t.Fatalf("expected zero expiry, got %s", expiresAt)
			}
		})
	}
}

func TestIsTokenUsable(t *testing.T) {
	now := time.Unix(2000, 0).UTC()
	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"empty", "", false},
		{"opaque", "token", true},
		{"valid JWT", testJWT(`{"exp":2001}`), true},
		{"expired JWT", testJWT(`{"exp":1999}`), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			usable, _, _ := IsTokenUsable(tt.token, now)
			if usable != tt.want {
				t.Fatalf("expected usable=%v, got %v", tt.want, usable)
			}
		})
	}
}

func testJWT(payload string) string {
	encodedHeader := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	encodedPayload := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return fmt.Sprintf("%s.%s.", encodedHeader, encodedPayload)
}

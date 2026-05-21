package config

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

type TokenExpiryStatus int

const (
	TokenExpiryNotInspectable TokenExpiryStatus = iota
	TokenExpiryValid
	TokenExpiryExpired
)

type jwtClaims struct {
	ExpiresAt json.Number `json:"exp"`
}

func InspectTokenExpiry(token string, now time.Time) (TokenExpiryStatus, time.Time) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return TokenExpiryNotInspectable, time.Time{}
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return TokenExpiryNotInspectable, time.Time{}
		}
	}

	var claims jwtClaims
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&claims); err != nil {
		return TokenExpiryNotInspectable, time.Time{}
	}
	if claims.ExpiresAt == "" {
		return TokenExpiryNotInspectable, time.Time{}
	}

	expUnix, err := claims.ExpiresAt.Int64()
	if err != nil {
		return TokenExpiryNotInspectable, time.Time{}
	}
	expiresAt := time.Unix(expUnix, 0).UTC()
	if !now.Before(expiresAt) {
		return TokenExpiryExpired, expiresAt
	}
	return TokenExpiryValid, expiresAt
}

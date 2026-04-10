package provider

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

type JWTPayload struct {
	Exp int64 `json:"exp,omitempty"`
	Iat int64 `json:"iat,omitempty"`
}

func ParseJWT(token string) (*JWTPayload, error) {
	parts := splitToken(token)
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid JWT format")
	}
	payload := parts[1]
	decoded, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, fmt.Errorf("decode JWT payload: %w", err)
	}
	var p JWTPayload
	if err := json.Unmarshal(decoded, &p); err != nil {
		return nil, fmt.Errorf("parse JWT payload: %w", err)
	}
	return &p, nil
}

func ExtractExpiryFromJWT(token string) int64 {
	p, err := ParseJWT(token)
	if err != nil || p.Exp == 0 {
		return 0
	}
	return p.Exp * 1000
}

func IsTokenValid(accessToken string, refreshToken string, expiryDate int64) bool {
	now := time.Now().UnixMilli()
	buffer := int64(30 * 60 * 1000)

	if expiryDate > 0 {
		return now < expiryDate-buffer
	}

	if refreshToken != "" {
		if p, err := ParseJWT(refreshToken); err == nil && p.Exp > 0 {
			return p.Exp*1000 > now
		}
	}

	if accessToken != "" {
		if p, err := ParseJWT(accessToken); err == nil && p.Exp > 0 {
			return now < p.Exp*1000-buffer
		}
	}

	return false
}

func splitToken(token string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	if start < len(token) {
		parts = append(parts, token[start:])
	}
	return parts
}

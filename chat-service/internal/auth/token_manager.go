package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
)

var ErrMissingJWTSecret = errors.New("JWT secret is empty")

type Claims struct {
	UserID int
	Role   string
	Exp    int64
	Type   string
}

type TokenManager struct {
	secret []byte
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{secret: []byte(secret)}
}

func (m *TokenManager) Parse(token string) (*Claims, error) {
	if len(m.secret) == 0 {
		return nil, ErrMissingJWTSecret
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	unsigned := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(unsigned))
	expected := mac.Sum(nil)

	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(expected, actual) {
		return nil, errors.New("invalid signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, err
	}

	var payload struct {
		Sub  string `json:"sub"`
		Role string `json:"role"`
		Exp  int64  `json:"exp"`
		Type string `json:"type"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, err
	}
	if payload.Sub == "" || payload.Role == "" || payload.Exp == 0 {
		return nil, errors.New("missing required token claims")
	}
	if time.Now().Unix() > payload.Exp {
		return nil, errors.New("token expired")
	}

	userID, err := strconv.Atoi(payload.Sub)
	if err != nil || userID <= 0 {
		return nil, errors.New("invalid token subject")
	}

	return &Claims{
		UserID: userID,
		Role:   payload.Role,
		Exp:    payload.Exp,
		Type:   payload.Type,
	}, nil
}

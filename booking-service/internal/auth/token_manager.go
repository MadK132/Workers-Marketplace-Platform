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
}

type TokenManager struct {
	secret []byte
}

func NewTokenManager(secret string) *TokenManager {
	return &TokenManager{
		secret: []byte(secret),
	}
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

	var raw map[string]any
	if err := json.Unmarshal(payloadBytes, &raw); err != nil {
		return nil, err
	}

	exp := int64(raw["exp"].(float64))
	if time.Now().Unix() > exp {
		return nil, errors.New("token expired")
	}

	userID, _ := strconv.Atoi(raw["sub"].(string))
	role, _ := raw["role"].(string)

	return &Claims{
		UserID: userID,
		Role:   role,
		Exp:    exp,
	}, nil
}

package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"diploma/usermanagement-service/internal/model"
)

var ErrMissingJWTSecret = errors.New("JWT secret is empty")

type TokenManager struct {
	secret []byte
	ttl    time.Duration
}

func NewTokenManager(secret string, ttl time.Duration) *TokenManager {
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &TokenManager{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (m *TokenManager) GenerateAccessToken(user model.User) (string, time.Time, error) {
	if len(m.secret) == 0 {
		return "", time.Time{}, ErrMissingJWTSecret
	}

	now := time.Now().UTC()
	expiresAt := now.Add(m.ttl)

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	claims := map[string]any{
		"sub":  strconv.Itoa(user.ID),
		"role": string(user.Role),
		"iat":  now.Unix(),
		"exp":  expiresAt.Unix(),
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", time.Time{}, err
	}
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", time.Time{}, err
	}

	unsignedToken := encodePart(headerJSON) + "." + encodePart(claimsJSON)

	mac := hmac.New(sha256.New, m.secret)
	_, _ = mac.Write([]byte(unsignedToken))
	signature := mac.Sum(nil)

	return unsignedToken + "." + encodePart(signature), expiresAt, nil
}

func encodePart(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}

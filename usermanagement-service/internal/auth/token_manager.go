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

	"diploma/usermanagement-service/internal/model"
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

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	unsigned := encode(headerJSON) + "." + encode(claimsJSON)

	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(unsigned))
	signature := mac.Sum(nil)

	token := unsigned + "." + encode(signature)

	return token, expiresAt, nil
}

func (m *TokenManager) Parse(token string) (*Claims, error) {
	claims, err := m.parse(token)
	if err != nil {
		return nil, err
	}
	if claims.Role == "" {
		return nil, errors.New("missing required token claims")
	}
	if claims.Type == "refresh" {
		return nil, errors.New("invalid token type")
	}
	return claims, nil
}

func (m *TokenManager) ParseRefreshToken(token string) (*Claims, error) {
	claims, err := m.parse(token)
	if err != nil {
		return nil, err
	}
	if claims.Type != "refresh" {
		return nil, errors.New("invalid token type")
	}
	return claims, nil
}

func (m *TokenManager) parse(token string) (*Claims, error) {
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

	if payload.Sub == "" || payload.Exp == 0 {
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

func encode(data []byte) string {
	return base64.RawURLEncoding.EncodeToString(data)
}
func (m *TokenManager) GenerateRefreshToken(user model.User) (string, time.Time, error) {
	if len(m.secret) == 0 {
		return "", time.Time{}, ErrMissingJWTSecret
	}

	now := time.Now().UTC()
	expiresAt := now.Add(7 * 24 * time.Hour)

	claims := map[string]any{
		"sub":  strconv.Itoa(user.ID),
		"type": "refresh",
		"exp":  expiresAt.Unix(),
	}

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	unsigned := encode(headerJSON) + "." + encode(claimsJSON)

	mac := hmac.New(sha256.New, m.secret)
	mac.Write([]byte(unsigned))
	signature := mac.Sum(nil)

	return unsigned + "." + encode(signature), expiresAt, nil
}

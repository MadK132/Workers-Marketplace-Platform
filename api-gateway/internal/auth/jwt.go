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

type Claims struct {
	UserID int
	Role   string
	Exp    int64
}

func ParseJWT(token, secret string) (*Claims, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is empty")
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid token format")
	}

	unsigned := parts[0] + "." + parts[1]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(unsigned))
	expected := mac.Sum(nil)

	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, errors.New("invalid token signature encoding")
	}
	if !hmac.Equal(expected, actual) {
		return nil, errors.New("invalid token signature")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("invalid token payload encoding")
	}

	var payload struct {
		Sub  string `json:"sub"`
		Role string `json:"role"`
		Exp  int64  `json:"exp"`
	}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, errors.New("invalid token payload")
	}

	if payload.Sub == "" || payload.Role == "" || payload.Exp == 0 {
		return nil, errors.New("missing required token claims")
	}

	userID, err := strconv.Atoi(payload.Sub)
	if err != nil || userID <= 0 {
		return nil, errors.New("invalid token subject")
	}

	if time.Now().Unix() > payload.Exp {
		return nil, errors.New("token expired")
	}

	return &Claims{
		UserID: userID,
		Role:   payload.Role,
		Exp:    payload.Exp,
	}, nil
}

package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DB   DBConfig
	HTTP HTTPConfig
	JWT  JWTConfig
}

type DBConfig struct {
	DSN         string
	MaxConns    int32
	MinConns    int32
	PingTimeout time.Duration
}

type HTTPConfig struct {
	Port string
}

type JWTConfig struct {
	Secret string
	TTL    time.Duration
}

func Load() Config {
	return Config{
		DB: DBConfig{
			DSN:         getEnv("DB_DSN", "postgres://user:pass@localhost:5433/app?sslmode=disable"),
			MaxConns:    getEnvInt32("DB_MAX_CONNS", 10),
			MinConns:    getEnvInt32("DB_MIN_CONNS", 2),
			PingTimeout: getEnvDuration("DB_PING_TIMEOUT", "3s"),
		},
		HTTP: HTTPConfig{
			Port: getEnv("HTTP_PORT", "8081"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "change-me"),
			TTL:    getEnvDuration("JWT_TTL", "15m"),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt32(key string, fallback int32) int32 {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return int32(i)
		}
	}
	return fallback
}

func getEnvDuration(key, fallback string) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	d, _ := time.ParseDuration(fallback)
	return d
}

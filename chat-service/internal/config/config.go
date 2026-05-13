package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTP    HTTPConfig
	DB      DBConfig
	JWT     JWTConfig
	Gateway GatewayConfig
	Redis   RedisConfig
}

type HTTPConfig struct {
	Port string
}

type DBConfig struct {
	DSN         string
	MaxConns    int32
	MinConns    int32
	PingTimeout time.Duration
}

type JWTConfig struct {
	Secret string
}

type GatewayConfig struct {
	SharedSecret string
}

type RedisConfig struct {
	Enabled  bool
	Addr     string
	Password string
	DB       int
	Channel  string
}

func Load() Config {
	return Config{
		HTTP: HTTPConfig{
			Port: getEnv("CHAT_HTTP_PORT", "8083"),
		},
		DB: DBConfig{
			DSN:         getEnv("DB_DSN", "postgres://user:pass@localhost:5433/app?sslmode=disable"),
			MaxConns:    getEnvInt32("DB_MAX_CONNS", 10),
			MinConns:    getEnvInt32("DB_MIN_CONNS", 2),
			PingTimeout: getEnvDuration("DB_PING_TIMEOUT", "3s"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
		},
		Gateway: GatewayConfig{
			SharedSecret: getEnv("GATEWAY_SHARED_SECRET", ""),
		},
		Redis: RedisConfig{
			Enabled:  getEnvBool("CHAT_REDIS_ENABLED", true),
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			Channel:  getEnv("CHAT_REDIS_CHANNEL", "chat.events"),
		},
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func getEnvInt32(key string, fallback int32) int32 {
	return int32(getEnvInt(key, int(fallback)))
}

func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
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

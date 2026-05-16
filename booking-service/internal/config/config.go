package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DB      DBConfig
	HTTP    HTTPConfig
	GRPC    GRPCConfig
	JWT     JWTConfig
	Gateway GatewayConfig
	User    UserServiceConfig
	Payment PaymentServiceConfig
}

type HTTPConfig struct {
	Port string
}

type GRPCConfig struct {
	Port string
}

type UserServiceConfig struct {
	URL string
}

type PaymentServiceConfig struct {
	GRPCAddress string
	Provider    string
	Currency    string
}

type JWTConfig struct {
	Secret string
}
type GatewayConfig struct {
	SharedSecret string
}

type DBConfig struct {
	DSN         string
	MaxConns    int32
	MinConns    int32
	PingTimeout time.Duration
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
			Port: getEnv("BOOKING_HTTP_PORT", "8082"),
		},
		GRPC: GRPCConfig{
			Port: getEnv("BOOKING_GRPC_PORT", "9094"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", ""),
		},
		Gateway: GatewayConfig{
			SharedSecret: getEnv("GATEWAY_SHARED_SECRET", ""),
		},
		User: UserServiceConfig{
			URL: getEnv("USER_SERVICE_URL", "http://localhost:8081"),
		},
		Payment: PaymentServiceConfig{
			GRPCAddress: getEnv("PAYMENT_GRPC_ADDR", "localhost:9096"),
			Provider:    getEnv("PAYMENT_PROVIDER", "stripe"),
			Currency:    getEnv("PAYMENT_CURRENCY", "KZT"),
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

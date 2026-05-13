package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DB       DBConfig
	GRPC     GRPCConfig
	Provider ProviderConfig
	Redis    RedisConfig
}

type DBConfig struct {
	DSN         string
	MaxConns    int32
	MinConns    int32
	PingTimeout time.Duration
}

type GRPCConfig struct {
	Port string
}

type ProviderConfig struct {
	DefaultProvider          string
	CloudPaymentsPublicID    string
	CloudPaymentsAPISecret   string
	CloudPaymentsCheckoutURL string
	KaspiMerchantID          string
	KaspiPaymentBaseURL      string
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
		DB: DBConfig{
			DSN:         getEnv("DB_DSN", "postgres://user:pass@localhost:5433/app?sslmode=disable"),
			MaxConns:    getEnvInt32("DB_MAX_CONNS", 10),
			MinConns:    getEnvInt32("DB_MIN_CONNS", 2),
			PingTimeout: getEnvDuration("DB_PING_TIMEOUT", "3s"),
		},
		GRPC: GRPCConfig{
			Port: getEnv("PAYMENT_GRPC_PORT", "9096"),
		},
		Provider: ProviderConfig{
			DefaultProvider:          getEnv("PAYMENT_PROVIDER", "cloudpayments_kaspi"),
			CloudPaymentsPublicID:    getEnv("CLOUDPAYMENTS_PUBLIC_ID", ""),
			CloudPaymentsAPISecret:   getEnv("CLOUDPAYMENTS_API_SECRET", ""),
			CloudPaymentsCheckoutURL: getEnv("CLOUDPAYMENTS_CHECKOUT_URL", "https://widget.cloudpayments.ru/bundles/cloudpayments"),
			KaspiMerchantID:          getEnv("KASPI_MERCHANT_ID", ""),
			KaspiPaymentBaseURL:      getEnv("KASPI_PAYMENT_BASE_URL", ""),
		},
		Redis: RedisConfig{
			Enabled:  getEnvBool("NOTIFICATION_REDIS_ENABLED", true),
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
			Channel:  getEnv("NOTIFICATION_REDIS_CHANNEL", "notification.events"),
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
	return int32(getEnvInt(key, int(fallback)))
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
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

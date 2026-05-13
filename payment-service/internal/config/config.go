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

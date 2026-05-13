package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DB       DBConfig
	GRPC     GRPCConfig
	HTTP     HTTPConfig
	Provider ProviderConfig
	Gateway  GatewayConfig
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

type HTTPConfig struct {
	WebhookPort string
}

type ProviderConfig struct {
	DefaultProvider      string
	StripeSecretKey      string
	StripeSuccessURL     string
	StripeCancelURL      string
	StripeCheckoutAPIURL string
	StripeMockURL        string
	StripeWebhookSecret  string
}

type GatewayConfig struct {
	SharedSecret string
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
		HTTP: HTTPConfig{
			WebhookPort: getEnv("PAYMENT_WEBHOOK_PORT", "8096"),
		},
		Provider: ProviderConfig{
			DefaultProvider:      getEnv("PAYMENT_PROVIDER", "stripe"),
			StripeSecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
			StripeSuccessURL:     getEnv("STRIPE_SUCCESS_URL", "http://localhost:5173/payment/success?session_id={CHECKOUT_SESSION_ID}"),
			StripeCancelURL:      getEnv("STRIPE_CANCEL_URL", "http://localhost:5173/payment/cancel"),
			StripeCheckoutAPIURL: getEnv("STRIPE_CHECKOUT_API_URL", "https://api.stripe.com/v1/checkout/sessions"),
			StripeMockURL:        getEnv("STRIPE_MOCK_URL", "http://localhost:5173/payment/mock"),
			StripeWebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
		},
		Gateway: GatewayConfig{
			SharedSecret: getEnv("GATEWAY_SHARED_SECRET", ""),
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

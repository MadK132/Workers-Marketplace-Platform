package config

import (
	"os"
	"strings"
)

type Config struct {
	Port              string
	UserServiceURL    string
	BookingServiceURL string
	JWTSecret         string
	GatewaySecret     string
	AllowedOrigins    []string
}

func Load() Config {
	return Config{
		Port:              getEnv("GATEWAY_PORT", "8080"),
		UserServiceURL:    getEnv("USER_SERVICE_URL", "http://localhost:8081"),
		BookingServiceURL: getEnv("BOOKING_SERVICE_URL", "http://localhost:8082"),
		JWTSecret:         getEnv("JWT_SECRET", "change-me"),
		GatewaySecret:     getEnv("GATEWAY_SHARED_SECRET", ""),
		AllowedOrigins: parseOrigins(
			getEnv(
				"CORS_ALLOWED_ORIGINS",
				"http://localhost:3000,http://localhost:5173",
			),
		),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		out = append(out, origin)
	}
	return out
}

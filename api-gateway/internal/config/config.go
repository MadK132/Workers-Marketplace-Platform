package config

import "os"

type Config struct {
	Port              string
	UserServiceURL    string
	BookingServiceURL string
	JWTSecret         string
	GatewaySecret     string
}

func Load() Config {
	return Config{
		Port:              getEnv("GATEWAY_PORT", "8080"),
		UserServiceURL:    getEnv("USER_SERVICE_URL", "http://localhost:8081"),
		BookingServiceURL: getEnv("BOOKING_SERVICE_URL", "http://localhost:8082"),
		JWTSecret:         getEnv("JWT_SECRET", "change-me"),
		GatewaySecret:     getEnv("GATEWAY_SHARED_SECRET", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

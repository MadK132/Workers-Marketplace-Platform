package main

import (
	"log"
	"net/url"

	"diploma/api-gateway/internal/config"
	"diploma/api-gateway/internal/proxy"
	"diploma/api-gateway/internal/router"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required")
	}

	userURL, err := url.Parse(cfg.UserServiceURL)
	if err != nil {
		log.Fatalf("invalid USER_SERVICE_URL: %v", err)
	}
	bookingURL, err := url.Parse(cfg.BookingServiceURL)
	if err != nil {
		log.Fatalf("invalid BOOKING_SERVICE_URL: %v", err)
	}
	chatURL, err := url.Parse(cfg.ChatServiceURL)
	if err != nil {
		log.Fatalf("invalid CHAT_SERVICE_URL: %v", err)
	}
	geoURL, err := url.Parse(cfg.GeoServiceURL)
	if err != nil {
		log.Fatalf("invalid GEOLOCATION_SERVICE_URL: %v", err)
	}
	notificationURL, err := url.Parse(cfg.NotificationURL)
	if err != nil {
		log.Fatalf("invalid NOTIFICATION_SERVICE_URL: %v", err)
	}

	userProxy := proxy.New(userURL)
	bookingProxy := proxy.New(bookingURL)
	chatProxy := proxy.New(chatURL)
	geoProxy := proxy.New(geoURL)
	notificationProxy := proxy.New(notificationURL)
	r := router.Setup(cfg, userProxy, bookingProxy, chatProxy, geoProxy, notificationProxy)

	log.Printf("API gateway listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

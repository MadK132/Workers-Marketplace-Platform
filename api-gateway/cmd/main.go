package main

import (
	"log"
	"net/url"

	"diploma/api-gateway/internal/config"
	"diploma/api-gateway/internal/proxy"
	"diploma/api-gateway/internal/router"
)

func main() {
	cfg := config.Load()

	userURL, err := url.Parse(cfg.UserServiceURL)
	if err != nil {
		log.Fatalf("invalid USER_SERVICE_URL: %v", err)
	}
	bookingURL, err := url.Parse(cfg.BookingServiceURL)
	if err != nil {
		log.Fatalf("invalid BOOKING_SERVICE_URL: %v", err)
	}

	userProxy := proxy.New(userURL)
	bookingProxy := proxy.New(bookingURL)
	r := router.Setup(cfg, userProxy, bookingProxy)

	log.Printf("API gateway listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}

package main

import (
	"context"
	"log"

	"diploma/booking-service/internal/auth"
	"diploma/booking-service/internal/client"
	"diploma/booking-service/internal/config"
	"diploma/booking-service/internal/db"
	"diploma/booking-service/internal/handler"
	"diploma/booking-service/internal/repository"
	"diploma/booking-service/internal/router"
	"diploma/booking-service/internal/service"
)

func main() {
	ctx := context.Background()

	cfg := config.Load()

	pool, err := db.NewPool(ctx, db.Config{
		DSN:         cfg.DB.DSN,
		MaxConns:    cfg.DB.MaxConns,
		MinConns:    cfg.DB.MinConns,
		PingTimeout: cfg.DB.PingTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}

	tokenManager := auth.NewTokenManager(cfg.JWT.Secret)

	requestRepo := repository.NewRequestRepository(pool)
	requestService := service.NewRequestService(requestRepo)
	bookingRepo := repository.NewBookingRepository(pool)
	bookingService := service.NewBookingService(bookingRepo)

	userClient := client.NewUserClient("http://localhost:8081")

	h := handler.NewHandler(requestService, bookingService, userClient)
	r := router.SetupRouter(h, tokenManager)

	log.Println("Booking service running on :8082")
	r.Run(":8082")
}

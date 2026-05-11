package main

import (
	"context"
	"log"
	"net"
	"net/http"

	bookingpb "diploma/api/booking-service-proto"
	"diploma/booking-service/internal/auth"
	"diploma/booking-service/internal/client"
	"diploma/booking-service/internal/config"
	"diploma/booking-service/internal/db"
	"diploma/booking-service/internal/grpcserver"
	"diploma/booking-service/internal/handler"
	"diploma/booking-service/internal/repository"
	"diploma/booking-service/internal/router"
	"diploma/booking-service/internal/service"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

func main() {
	ctx := context.Background()

	_ = godotenv.Load()
	cfg := config.Load()
	if cfg.JWT.Secret == "" {
		log.Fatal("JWT_SECRET is required")
	}

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
	r := router.SetupRouter(h, tokenManager, cfg.Gateway.SharedSecret)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalf("gRPC listen error: %v", err)
	}
	grpcServer := grpc.NewServer()
	bookingpb.RegisterBookingServiceServer(
		grpcServer,
		grpcserver.New(requestService, bookingService),
	)

	go func() {
		log.Printf("Booking gRPC server listening on :%s", cfg.GRPC.Port)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	log.Println("Booking service running on :8082")
	if err := r.Run(":8082"); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	geolocationpb "diploma/api/geolocation-service-proto"
	"diploma/geolocation-service/internal/config"
	"diploma/geolocation-service/internal/db"
	"diploma/geolocation-service/internal/grpcserver"
	"diploma/geolocation-service/internal/handler"
	"diploma/geolocation-service/internal/repository"
	"diploma/geolocation-service/internal/router"
	"diploma/geolocation-service/internal/service"

	"github.com/joho/godotenv"
	"google.golang.org/grpc"
)

func main() {
	ctx := context.Background()

	_ = godotenv.Load()
	cfg := config.Load()

	pool, err := db.NewPool(ctx, db.Config{
		DSN:         cfg.DB.DSN,
		MaxConns:    cfg.DB.MaxConns,
		MinConns:    cfg.DB.MinConns,
		PingTimeout: cfg.DB.PingTimeout,
	})
	if err != nil {
		log.Fatal("DB connection error:", err)
	}
	defer pool.Close()

	geoRepo := repository.NewGeolocationRepository(pool)
	geoService := service.NewGeolocationService(geoRepo)
	geoHandler := handler.New(geoService)

	r := router.Setup(geoHandler, cfg)

	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalf("gRPC listen error: %v", err)
	}
	grpcServer := grpc.NewServer()
	geolocationpb.RegisterGeolocationServiceServer(
		grpcServer,
		grpcserver.New(geoService),
	)

	httpServer := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Geolocation HTTP service listening on :%s", cfg.HTTP.Port)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	go func() {
		log.Printf("Geolocation gRPC service listening on :%s", cfg.GRPC.Port)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP graceful shutdown failed: %v", err)
	}
	grpcServer.GracefulStop()

	log.Println("Geolocation service stopped")
}

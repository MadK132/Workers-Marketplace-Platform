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

	paymentpb "diploma/api/payment-service-proto"
	"diploma/payment-service/internal/config"
	"diploma/payment-service/internal/db"
	"diploma/payment-service/internal/grpcmiddleware"
	"diploma/payment-service/internal/grpcserver"
	"diploma/payment-service/internal/provider"
	"diploma/payment-service/internal/repository"
	"diploma/payment-service/internal/service"

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

	paymentRepo := repository.NewPaymentRepository(pool)
	if err := paymentRepo.EnsureTable(ctx); err != nil {
		log.Fatalf("Payment schema bootstrap error: %v", err)
	}
	paymentProvider := provider.New(provider.Config{
		DefaultProvider:      cfg.Provider.DefaultProvider,
		StripeSecretKey:      cfg.Provider.StripeSecretKey,
		StripeSuccessURL:     cfg.Provider.StripeSuccessURL,
		StripeCancelURL:      cfg.Provider.StripeCancelURL,
		StripeCheckoutAPIURL: cfg.Provider.StripeCheckoutAPIURL,
		StripeMockURL:        cfg.Provider.StripeMockURL,
	})
	paymentService := service.NewPaymentService(paymentRepo, paymentProvider)

	listener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalf("gRPC listen error: %v", err)
	}

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(
		grpcmiddleware.Auth(cfg.Gateway.SharedSecret),
	))
	paymentpb.RegisterPaymentServiceServer(
		grpcServer,
		grpcserver.New(paymentService),
	)

	go func() {
		log.Printf("Payment gRPC service listening on :%s", cfg.GRPC.Port)
		if err := grpcServer.Serve(listener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
		}
	}()

	webhookServer := &http.Server{
		Addr:              ":" + cfg.HTTP.WebhookPort,
		Handler:           grpcserver.NewWebhookHandler(paymentService, cfg.Provider.StripeWebhookSecret),
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Printf("Payment webhook service listening on :%s", cfg.HTTP.WebhookPort)
		if err := webhookServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("webhook server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := webhookServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Payment webhook graceful shutdown failed: %v", err)
	}
	grpcServer.GracefulStop()
	log.Println("Payment service stopped")
}

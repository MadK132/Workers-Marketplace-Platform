package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	paymentpb "diploma/api/payment-service-proto"
	"diploma/payment-service/internal/config"
	"diploma/payment-service/internal/db"
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
	paymentProvider := provider.New(provider.Config{
		DefaultProvider:          cfg.Provider.DefaultProvider,
		CloudPaymentsPublicID:    cfg.Provider.CloudPaymentsPublicID,
		CloudPaymentsCheckoutURL: cfg.Provider.CloudPaymentsCheckoutURL,
		KaspiMerchantID:          cfg.Provider.KaspiMerchantID,
		KaspiPaymentBaseURL:      cfg.Provider.KaspiPaymentBaseURL,
	})
	paymentService := service.NewPaymentService(paymentRepo, paymentProvider)

	listener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalf("gRPC listen error: %v", err)
	}

	grpcServer := grpc.NewServer()
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

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	grpcServer.GracefulStop()
	log.Println("Payment service stopped")
}

package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	paymentpb "diploma/api/payment-service-proto"
	"diploma/internal/notifications"
	"diploma/payment-service/internal/config"
	"diploma/payment-service/internal/db"
	"diploma/payment-service/internal/grpcserver"
	"diploma/payment-service/internal/provider"
	"diploma/payment-service/internal/repository"
	"diploma/payment-service/internal/service"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
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
	notifier := notifications.Publisher(notifications.NoopPublisher{})
	var redisClient *redis.Client
	if cfg.Redis.Enabled {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("Redis disabled for payment notifications: %v", err)
		} else {
			notifier = notifications.NewRedisPublisher(redisClient, cfg.Redis.Channel)
			log.Printf("Payment notification publisher enabled on %s channel %s", cfg.Redis.Addr, cfg.Redis.Channel)
		}
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	paymentProvider := provider.New(provider.Config{
		DefaultProvider:          cfg.Provider.DefaultProvider,
		CloudPaymentsPublicID:    cfg.Provider.CloudPaymentsPublicID,
		CloudPaymentsCheckoutURL: cfg.Provider.CloudPaymentsCheckoutURL,
		KaspiMerchantID:          cfg.Provider.KaspiMerchantID,
		KaspiPaymentBaseURL:      cfg.Provider.KaspiPaymentBaseURL,
	})
	paymentService := service.NewPaymentService(paymentRepo, paymentProvider, notifier)

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

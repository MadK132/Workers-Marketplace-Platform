package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"diploma/notification-service/internal/auth"
	"diploma/notification-service/internal/config"
	"diploma/notification-service/internal/db"
	"diploma/notification-service/internal/handler"
	"diploma/notification-service/internal/realtime"
	"diploma/notification-service/internal/repository"
	"diploma/notification-service/internal/router"
	notificationservice "diploma/notification-service/internal/service"

	"github.com/joho/godotenv"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		log.Fatal("DB connection error:", err)
	}
	defer pool.Close()

	notificationRepo := repository.NewNotificationRepository(pool)
	if err := notificationRepo.EnsureSchema(ctx); err != nil {
		log.Fatal("notification schema bootstrap error:", err)
	}

	notificationService := notificationservice.NewNotificationService(notificationRepo)
	hub := realtime.NewHub()

	var redisClient *redis.Client
	if cfg.Redis.Enabled {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("Redis disabled for notification events: %v", err)
		} else {
			consumer := realtime.NewRedisConsumer(
				redisClient,
				cfg.Redis.Channel,
				notificationService,
				hub,
			)
			go consumer.Run(ctx)
			log.Printf("Notification Redis consumer listening on %s channel %s", cfg.Redis.Addr, cfg.Redis.Channel)
		}
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	tokenManager := auth.NewTokenManager(cfg.JWT.Secret)
	notificationHandler := handler.New(notificationService, hub)
	r := router.Setup(notificationHandler, tokenManager, cfg.Gateway.SharedSecret)

	server := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Notification service listening on :%s", cfg.HTTP.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down notification service...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	}

	log.Println("Notification service stopped")
}

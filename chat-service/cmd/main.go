package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"diploma/chat-service/internal/auth"
	"diploma/chat-service/internal/config"
	"diploma/chat-service/internal/db"
	"diploma/chat-service/internal/handler"
	"diploma/chat-service/internal/realtime"
	"diploma/chat-service/internal/repository"
	"diploma/chat-service/internal/router"
	chatservice "diploma/chat-service/internal/service"
	"diploma/internal/notifications"

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

	chatRepo := repository.NewChatRepository(pool)
	if err := chatRepo.EnsureSchema(ctx); err != nil {
		log.Fatal("chat schema bootstrap error:", err)
	}

	hub := realtime.NewHub()
	publisher := realtime.Publisher(realtime.NoopPublisher{})
	notifier := notifications.Publisher(notifications.NoopPublisher{})
	var redisClient *redis.Client

	if cfg.Redis.Enabled {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})

		if err := redisClient.Ping(ctx).Err(); err != nil {
			log.Printf("Redis disabled for chat pub/sub: %v", err)
		} else {
			nodeID := fmt.Sprintf("%s-%d", hostname(), os.Getpid())
			bus := realtime.NewRedisBus(redisClient, cfg.Redis.Channel, nodeID)
			publisher = bus
			notifier = notifications.NewRedisPublisher(redisClient, cfg.Notifications.Channel)
			go bus.Run(ctx, hub)
			log.Printf("Chat Redis pub/sub enabled on %s channel %s", cfg.Redis.Addr, cfg.Redis.Channel)
		}
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	tokenManager := auth.NewTokenManager(cfg.JWT.Secret)
	chatService := chatservice.NewChatService(chatRepo)
	chatHandler := handler.NewHandler(chatService, hub, publisher, notifier)
	r := router.SetupRouter(chatHandler, tokenManager, cfg.Gateway.SharedSecret)

	server := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("Chat service listening on :%s", cfg.HTTP.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down chat service...")
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	}

	log.Println("Chat service stopped")
}

func hostname() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "chat-service"
	}
	return name
}

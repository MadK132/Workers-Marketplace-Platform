package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"diploma/notification-service/internal/config"
	"diploma/notification-service/internal/db"
	"diploma/notification-service/internal/handler"
	"diploma/notification-service/internal/repository"
	"diploma/notification-service/internal/router"
	"diploma/notification-service/internal/service"

	"github.com/joho/godotenv"
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

	notificationRepo := repository.NewNotificationRepository(pool)
	if err := notificationRepo.EnsureSchema(ctx); err != nil {
		log.Fatal("notification schema bootstrap error:", err)
	}

	notificationService := service.NewNotificationService(notificationRepo)
	notificationHandler := handler.New(notificationService)
	r := router.Setup(notificationHandler, cfg)

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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	}

	log.Println("Notification service stopped")
}

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"diploma/usermanagement-service/internal/auth"
	"diploma/usermanagement-service/internal/config"
	"diploma/usermanagement-service/internal/db"
	"diploma/usermanagement-service/internal/handler"
	"diploma/usermanagement-service/internal/repository"
	"diploma/usermanagement-service/internal/router"
	"diploma/usermanagement-service/internal/service"
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
		log.Fatal("DB connection error:", err)
	}
	defer pool.Close()

	log.Println("DB connected successfully")

	userRepo := repository.NewUserRepository(pool)
	verificationRepo := repository.NewEmailVerificationRepository(pool)

	tokenManager := auth.NewTokenManager(cfg.JWT.Secret, cfg.JWT.TTL)

	authService := service.NewAuthService(
		userRepo,
		tokenManager,
		verificationRepo,
	)

	authHandler := handler.NewAuthHandler(authService)

	r := router.SetupRouter(authHandler, tokenManager)

	server := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("User management service listening on :%s", cfg.HTTP.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	log.Println("Shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Graceful shutdown failed: %v", err)
	}

	log.Println("Server stopped")
}

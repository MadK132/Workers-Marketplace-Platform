package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	usermanagementpb "diploma/api/usermanagement-service-proto"
	"diploma/usermanagement-service/internal/auth"
	"diploma/usermanagement-service/internal/config"
	"diploma/usermanagement-service/internal/db"
	"diploma/usermanagement-service/internal/email"
	"diploma/usermanagement-service/internal/grpcmiddleware"
	"diploma/usermanagement-service/internal/grpcserver"
	"diploma/usermanagement-service/internal/handler"
	"diploma/usermanagement-service/internal/repository"
	"diploma/usermanagement-service/internal/router"
	"diploma/usermanagement-service/internal/service"

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
		log.Fatal("DB connection error:", err)
	}
	defer pool.Close()

	log.Println("DB connected successfully")

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPass := os.Getenv("SMTP_PASS")

	smtpPort, _ := strconv.Atoi(smtpPortStr)

	emailSender := email.NewSender(
		smtpHost,
		smtpPort,
		smtpUser,
		smtpPass,
	)
	userRepo := repository.NewUserRepository(pool)
	verificationRepo := repository.NewEmailVerificationRepository(pool)
	passwordResetRepo := repository.NewPasswordResetRepository(pool)
	customerProfileRepo := repository.NewCustomerProfileRepository(pool)
	workerProfileRepo := repository.NewWorkerProfileRepository(pool)
	workerSkillRepo := repository.NewWorkerSkillRepository(pool)
	categoryRepo := repository.NewCategoryRepository(pool)
	adminRepo := repository.NewAdminRepository(pool)
	paymentMethodRepo := repository.NewPaymentMethodRepository(pool)

	if err := userRepo.EnsureManagerRole(ctx); err != nil {
		log.Printf("Manager role bootstrap skipped: %v", err)
	}
	if err := categoryRepo.EnsureDefaults(ctx); err != nil {
		log.Printf("Category bootstrap skipped: %v", err)
	}
	if err := workerSkillRepo.EnsureEvidenceTable(ctx); err != nil {
		log.Printf("Worker skill evidence bootstrap skipped: %v", err)
	}
	if err := paymentMethodRepo.EnsureTable(ctx); err != nil {
		log.Printf("Payment method bootstrap skipped: %v", err)
	}

	tokenManager := auth.NewTokenManager(cfg.JWT.Secret, cfg.JWT.TTL)
	authService := service.NewAuthService(
		userRepo,
		tokenManager,
		verificationRepo,
		emailSender,
		passwordResetRepo,
		customerProfileRepo,
		workerProfileRepo,
		workerSkillRepo,
		categoryRepo,
		adminRepo,
		paymentMethodRepo,
	)
	if err := authService.EnsureDefaultAdmin(ctx); err != nil {
		log.Fatalf("Default admin bootstrap error: %v", err)
	}

	authHandler := handler.NewAuthHandler(authService)

	r := router.SetupRouter(authHandler, tokenManager, cfg.Gateway.SharedSecret)
	grpcListener, err := net.Listen("tcp", ":"+cfg.GRPC.Port)
	if err != nil {
		log.Fatalf("gRPC listen error: %v", err)
	}
	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(
		grpcmiddleware.Auth(cfg.Gateway.SharedSecret),
	))
	usermanagementpb.RegisterUserManagementServiceServer(
		grpcServer,
		grpcserver.New(authService),
	)

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

	go func() {
		log.Printf("User management gRPC server listening on :%s", cfg.GRPC.Port)
		if err := grpcServer.Serve(grpcListener); err != nil {
			log.Fatalf("gRPC server error: %v", err)
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
	grpcServer.GracefulStop()

	log.Println("Server stopped")
}

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"otp-service/internal/auth"
	"otp-service/internal/config"
	"otp-service/internal/database"
	"otp-service/internal/delivery"
	"otp-service/internal/handler"
	"otp-service/internal/otp"
	"otp-service/internal/redis"
	"otp-service/internal/worker"
	"syscall"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	settings, err := config.Load()
	if err != nil {
		return err
	}

	appContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	connectContext, cancelConnect := context.WithTimeout(appContext, 10*time.Second)
	defer cancelConnect()

	pool, err := database.NewPool(connectContext, settings)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Initialize Redis client and rate limiter
	redisClient := redis.NewClient(settings)
	defer redisClient.Close()
	rateLimiter := redis.NewLimiter(redisClient, redis.DefaultConfig())

	// Verify Redis connectivity
	if err := redisClient.Ping(connectContext); err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	// Initialize worker pool for async OTP delivery
	workerPool := worker.NewWorkerPool(worker.WorkerPoolConfig{
		QueueCapacity: 100,
		WorkerCount:   4,
		MaxBackoff:    30 * time.Second,
		MaxRetries:    3,
		Delivery:      delivery.NoOp{},
	})

	if err := workerPool.Start(appContext); err != nil {
		return fmt.Errorf("start worker pool: %w", err)
	}
	defer workerPool.Shutdown(connectContext)

	// Initialize auth store and middleware
	authStore := auth.NewStore(pool)
	authMiddleware := auth.Middleware(authStore)

	otpRepository := otp.NewPostgresRepository(pool)
	otpService := otp.NewService(otpRepository, workerPool)

	otpHandler := handler.NewOTPHandler(otpService)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.HealthHandler)

	// Apply auth middleware, then rate limiting middleware to OTP endpoints
	mux.Handle("/v1/otp/send", authMiddleware(rateLimiter.Middleware(http.HandlerFunc(otpHandler.Send))))
	mux.Handle("/v1/otp/verify", authMiddleware(rateLimiter.Middleware(http.HandlerFunc(otpHandler.Verify))))

	server := &http.Server{
		Addr:    settings.HTTPAddr,
		Handler: mux,
	}

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("run HTTP server: %w", err)
		}
		return nil
	case <-appContext.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancelShutdown()

		if err := server.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shut down HTTP server: %w", err)
		}
		return nil
	}
}

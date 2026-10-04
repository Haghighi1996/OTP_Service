package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os/signal"
	"otp-service/internal/config"
	"otp-service/internal/database"
	"otp-service/internal/handler"
	"otp-service/internal/otp"
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

	otpRepository := otp.NewPostgresRepository(pool)
	otpService := otp.NewService(otpRepository)
	otpHandler := handler.NewOTPHandler(otpService)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handler.HealthHandler)
	mux.HandleFunc("/v1/otp/send", otpHandler.Send)
	mux.HandleFunc("/v1/otp/verify", otpHandler.Verify)

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

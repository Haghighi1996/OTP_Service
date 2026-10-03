package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"otp-service/internal/handler"
	"syscall"
	"time"
)

func main() {

	http.HandleFunc("/healthz", handler.HealthHandler)
	http.HandleFunc("/v1/otp/send", handler.SendOTPHandler)

	server := &http.Server{
		Addr:    ":8080",
		Handler: http.DefaultServeMux,
	}
	go server.ListenAndServe()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
	<-sigchan
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		// handle error
	}
}

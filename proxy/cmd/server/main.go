package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/time/rate"

	"semantic-cache-proxy/internal/cache"
	"semantic-cache-proxy/internal/proxy"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func main() {
	// Structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	// Connect to the Python Indexer gRPC server
	indexerAddr := os.Getenv("INDEXER_ADDR")
	if indexerAddr == "" {
		indexerAddr = "localhost:50051"
	}
	cacheClient, err := cache.NewClient(indexerAddr)
	if err != nil {
		slog.Error("Failed to connect to indexer", "addr", indexerAddr, "error", err)
		os.Exit(1)
	}
	defer cacheClient.Close()
	slog.Info("Connected to Python Indexer", "addr", indexerAddr)

	// Create the proxy handler with the cache client
	proxyHandler, err := proxy.NewHandler(cacheClient)
	if err != nil {
		slog.Error("Failed to create proxy handler", "error", err)
		os.Exit(1)
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)

	// Rate limiter: 10 req/s per IP, burst 20, cleanup stale entries every 10 min
	rl := proxy.NewRateLimiter(rate.Limit(10), 20, 10*time.Minute)
	mux.HandleFunc("/chat", rl.Middleware(proxyHandler.HandleChat))

	// Configure HTTP server
	addr := ":8080"
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan
		slog.Info("Received signal, initiating graceful shutdown", "signal", sig)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			slog.Error("Shutdown error", "error", err)
		}
	}()

	slog.Info("Go Proxy starting", "addr", addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		slog.Error("Server failed", "error", err)
		os.Exit(1)
	}

	slog.Info("Server stopped gracefully")
}

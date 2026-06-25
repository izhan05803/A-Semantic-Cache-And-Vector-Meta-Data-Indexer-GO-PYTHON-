package main

import (
	"log"
	"net/http"

	"semantic-cache-proxy/internal/cache"
	"semantic-cache-proxy/internal/proxy"
)

func main() {
	// Connect to the Python Indexer gRPC server
	indexerAddr := "localhost:50051"
	cacheClient, err := cache.NewClient(indexerAddr)
	if err != nil {
		log.Fatalf("Failed to connect to indexer at %s: %v", indexerAddr, err)
	}
	defer cacheClient.Close()
	log.Printf("Connected to Python Indexer at %s", indexerAddr)

	// Create the proxy handler with the cache client
	proxyHandler, err := proxy.NewHandler(cacheClient)
	if err != nil {
		log.Fatalf("Failed to create proxy handler: %v", err)
	}

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/chat", proxyHandler.HandleChat)

	// Start the HTTP proxy server
	addr := ":8080"
	log.Printf("Go Proxy starting on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

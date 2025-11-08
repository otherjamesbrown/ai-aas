package main

import (
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/otherjamesbrown/ai-aas/services/world-service/pkg/world"
)

func main() {
	addr := defaultAddr()
	mux := http.NewServeMux()
	mux.HandleFunc("/world", world.Handler)

	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	log.Printf("world-service listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server error: %v", err)
	}
}

func defaultAddr() string {
	if addr := os.Getenv("WORLD_SERVICE_ADDR"); addr != "" {
		return addr
	}
	return ":8080"
}

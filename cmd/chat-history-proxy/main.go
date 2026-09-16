// Command chat-history-proxy runs a read-only chat history proxy in front
// of Qiscus (see docs/CHAT_HISTORY_PROXY.md). It has no database: it
// exchanges a client-issued JWT for Qiscus server credentials and returns
// filtered, per-user results. Generic by design — every client-specific
// value (Qiscus app ID/secret, JWT public key, port, cache TTL) comes from
// environment variables (see config.Load), never from code.
package main

import (
	"crypto/rsa"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-jwt/jwt/v5"

	sharedmiddleware "github.com/amed12/chat-history-proxy/internal/middleware"
	"github.com/amed12/chat-history-proxy/internal/proxy/cache"
	"github.com/amed12/chat-history-proxy/internal/proxy/config"
	"github.com/amed12/chat-history-proxy/internal/proxy/handler"
	proxymiddleware "github.com/amed12/chat-history-proxy/internal/proxy/middleware"
	"github.com/amed12/chat-history-proxy/internal/proxy/qiscus"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	publicKey, err := jwt.ParseRSAPublicKeyFromPEM([]byte(cfg.JWTPublicKeyPEM))
	if err != nil {
		log.Fatalf("config: JWT_PUBLIC_KEY is not a valid RSA public key PEM: %v", err)
	}

	router := buildRouter(cfg, publicKey)

	log.Printf("chat-history-proxy starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

func buildRouter(cfg config.Config, publicKey *rsa.PublicKey) http.Handler {
	qiscusClient := qiscus.NewClient(cfg.QiscusAppID, cfg.QiscusSecretKey, cfg.QiscusBaseURL)
	sessionCache := cache.New[[]qiscus.Session](
		durationFromSeconds(cfg.CacheTTLSeconds),
	)

	sessionsHandler := &handler.SessionsHandler{Lister: qiscusClient, Cache: sessionCache}
	messagesHandler := &handler.MessagesHandler{Lister: qiscusClient, Cache: sessionCache, Fetcher: qiscusClient}

	r := chi.NewRouter()
	r.Use(chimiddleware.Recoverer)
	r.Use(sharedmiddleware.Logger)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	r.Group(func(r chi.Router) {
		r.Use(proxymiddleware.JWTAuthRS256(publicKey))
		r.Get("/api/v1/sessions", sessionsHandler.ServeHTTP)
		r.Get("/api/v1/sessions/{room_id}/messages", messagesHandler.ServeHTTP)
	})

	return r
}

func durationFromSeconds(seconds int) time.Duration {
	return time.Duration(seconds) * time.Second
}

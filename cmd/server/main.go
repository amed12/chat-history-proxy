package main

import (
    "log"
    "net/http"

    "github.com/go-chi/chi/v5"
    chimiddleware "github.com/go-chi/chi/v5/middleware"

    "github.com/hellogod/chat-history-service/internal/archive"
    "github.com/hellogod/chat-history-service/internal/config"
    "github.com/hellogod/chat-history-service/internal/database"
    "github.com/hellogod/chat-history-service/internal/handler"
    "github.com/hellogod/chat-history-service/internal/middleware"
    "github.com/hellogod/chat-history-service/internal/qiscus"
)

func main() {
    cfg := config.Load()

    // Database
    db := database.Open(cfg.DatabaseURL)
    defer db.Close()

    // Dependencies
    qiscusClient := qiscus.NewClient(cfg.QiscusAppID, cfg.QiscusSecretKey, cfg.QiscusBaseURL)
    repo := archive.NewRepository(db)
    svc := archive.NewService(repo, qiscusClient, cfg.QiscusAppID)

    // Handlers
    webhookHandler := handler.NewWebhookHandler(cfg.WebhookSecret, svc)
    historyHandler := handler.NewHistoryHandler(svc)
    authHandler := handler.NewAuthHandler(cfg.JWTSecret, cfg.JWTExpiryHours)

    // Router
    r := chi.NewRouter()
    r.Use(chimiddleware.Recoverer)
    r.Use(middleware.Logger)

    // Public routes
    r.Post("/webhook/resolve", webhookHandler.ServeHTTP)
    r.Post("/api/v1/auth/token", authHandler.ServeHTTP)

    // Protected routes
    r.Group(func(r chi.Router) {
        r.Use(middleware.JWTAuth(cfg.JWTSecret))
        r.Get("/api/v1/chat-history/{room_id}", historyHandler.ServeHTTP)
    })

    // Health check
    r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte(`{"status":"ok"}`))
    })

    log.Printf("server starting on :%s", cfg.Port)
    if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
        log.Fatalf("server error: %v", err)
    }
}

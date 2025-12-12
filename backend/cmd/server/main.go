package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"microapi/internal/auth"
	"microapi/internal/handlers"
	"microapi/internal/ratelimit"
	"microapi/internal/storage"
	"microapi/internal/stripecli"
)

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatal().Msgf("missing env %s", key)
	}
	return v
}

func main() {
	// logger
	levelStr := os.Getenv("LOG_LEVEL")
	if levelStr == "" { levelStr = "info" }
	lvl, _ := zerolog.ParseLevel(levelStr)
	zerolog.SetGlobalLevel(lvl)

	// load config
	dbURL := mustEnv("DATABASE_URL")
	redisAddr := mustEnv("REDIS_ADDR")
	redisPass := os.Getenv("REDIS_PASS")
	port := os.Getenv("PORT")
	if port == "" { port = "8080" }
	adminKey := mustEnv("ADMIN_API_KEY")

	// storage
	sdb, err := storage.NewPostgres(dbURL)
	if err != nil { log.Fatal().Err(err).Msg("pg connect") }
	defer sdb.Close(context.Background())

	// redis
	rlClient, err := ratelimit.NewRedisLimiter(redisAddr, redisPass)
	if err != nil { log.Fatal().Err(err).Msg("redis connect") }

	// stripe
	stripeApiKey := mustEnv("STRIPE_API_KEY")
	stripe := stripecli.New(stripeApiKey)

	// services
	authSvc := auth.NewService(sdb)
	handlerSvc := handlers.NewHandler(sdb, rlClient, authSvc, stripe, adminKey)

	r := chi.NewRouter()
	r.Use(handlerSvc.LoggingMiddleware)

	// public API endpoints (tenant APIs)
	r.Post("/v1/transform", handlerSvc.APIHandler("transform"))
	r.Post("/v1/summarize", handlerSvc.APIHandler("summarize"))
	r.Post("/v1/clean", handlerSvc.APIHandler("clean"))
	r.Post("/v1/generate", handlerSvc.APIHandler("generate"))
	r.Post("/v1/convert", handlerSvc.APIHandler("convert"))
	r.Post("/v1/format", handlerSvc.APIHandler("format"))
	r.Post("/v1/validate", handlerSvc.APIHandler("validate"))
	r.Post("/v1/storage", handlerSvc.APIHandler("storage"))
	r.Get("/v1/metrics", handlerSvc.MetricsHandler())
	r.Post("/v1/mock", handlerSvc.APIHandler("mock"))

	// admin endpoints
	r.Group(func(r chi.Router){
		r.Use(handlerSvc.AdminAuthMiddleware(adminKey))
		r.Post("/admin/tenants", handlerSvc.CreateTenant)
		r.Post("/admin/api_keys", handlerSvc.CreateAPIKey) // returns plaintext key once
		r.Post("/admin/assign_stripe", handlerSvc.AssignStripeCustomer)
		r.Get("/admin/usage/{tenant_id}", handlerSvc.GetUsage)
	})

	// stripe webhook
	r.Post("/webhook/stripe", handlerSvc.StripeWebhookHandler())

	addr := ":" + port
	log.Info().Msgf("starting server on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal().Err(err).Msg("server failed")
	}
}

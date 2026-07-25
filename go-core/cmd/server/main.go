package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/meysam81/go-auth/auth/basic"
	authjwt "github.com/meysam81/go-auth/auth/jwt"

	"tinder-core/internal/config"
	"tinder-core/internal/events"
	"tinder-core/internal/platform/postgres"
	"tinder-core/internal/platform/redis"
	httptransport "tinder-core/internal/transport/http"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	if len(cfg.JWTSigningKey) < 32 {
		log.Fatal("JWT_SIGNING_KEY must contain at least 32 characters")
	}

	db, err := postgres.New(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}
	defer db.Close()

	authStore := postgres.NewAuthStore(db)
	authenticator, err := basic.NewAuthenticator(basic.Config{
		UserStore:       authStore,
		CredentialStore: authStore,
	})
	if err != nil {
		log.Fatalf("authenticator: %v", err)
	}

	tokenManager, err := authjwt.NewTokenManager(authjwt.Config{
		UserStore:      authStore,
		SigningKey:     []byte(cfg.JWTSigningKey),
		Issuer:         cfg.JWTIssuer,
		AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		log.Fatalf("token manager: %v", err)
	}

	redisClient, err := redis.New(cfg.RedisAddr)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}
	defer redisClient.Close()

	publisher := events.NewPublisher(redisClient, cfg.RedisStream, cfg.PublisherPool, 256)
	publisher.Start(ctx, cfg.PublisherPool)
	defer publisher.Close()

	router := httptransport.NewRouter(httptransport.Deps{
		Publisher:     publisher,
		Authenticator: authenticator,
		TokenManager:  tokenManager,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		log.Printf("tinder-core listening on :%s", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("http shutdown: %v", err)
	}
}

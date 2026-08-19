package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/meysam81/go-auth/auth/basic"
	authjwt "github.com/meysam81/go-auth/auth/jwt"

	"tinder-core/internal/config"
	"tinder-core/internal/events"
	"tinder-core/internal/platform/logging"
	"tinder-core/internal/platform/objectstorage"
	"tinder-core/internal/platform/postgres"
	"tinder-core/internal/platform/redis"
	"tinder-core/internal/repository"
	"tinder-core/internal/service"
	httptransport "tinder-core/internal/transport/http"
	"tinder-core/internal/transport/http/handlers"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg := config.Load()
	logger := logging.New(cfg.Env, cfg.LogLevel)
	slog.SetDefault(logger)

	if len(cfg.JWTSigningKey) < 32 {
		logger.Error("invalid configuration", "error", "JWT_SIGNING_KEY must contain at least 32 characters")
		os.Exit(1)
	}

	db, err := postgres.New(cfg.PostgresDSN)
	if err != nil {
		logger.Error("connect to Postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	if err := postgres.ApplyMigrations(ctx, db); err != nil {
		logger.Error("apply database migrations", "error", err)
		os.Exit(1)
	}
	logger.Info("database migrations are up to date")

	userRepository := repository.NewUserRepository(db)
	profileRepository := repository.NewProfileRepository(db)
	ribbonRepository := repository.NewRibbonRepository(db)
	chatRepository := repository.NewChatRepository(db)
	photoRepository := repository.NewPhotoRepository(db)
	profileService := service.NewProfileService(profileRepository)
	ribbonService := service.NewRibbonService(ribbonRepository, profileRepository, logger)
	chatService := service.NewChatService(chatRepository)
	authStore := postgres.NewAuthStore(db, userRepository)
	authenticator, err := basic.NewAuthenticator(basic.Config{
		UserStore:       authStore,
		CredentialStore: authStore,
	})
	if err != nil {
		logger.Error("create authenticator", "error", err)
		os.Exit(1)
	}
	tokenManager, err := authjwt.NewTokenManager(authjwt.Config{
		UserStore:      authStore,
		SigningKey:     []byte(cfg.JWTSigningKey),
		Issuer:         cfg.JWTIssuer,
		AccessTokenTTL: 15 * time.Minute,
	})
	if err != nil {
		logger.Error("create token manager", "error", err)
		os.Exit(1)
	}
	authHandler := handlers.NewAuthHandler(authenticator, tokenManager, logger)
	profileHandler := handlers.NewProfileHandler(profileService)
	ribbonHandler := handlers.NewRibbonHandler(ribbonService, ribbonRepository)
	chatHandler := handlers.NewChatHandler(chatService)

	storageCtx, cancelStorage := context.WithTimeout(ctx, 10*time.Second)
	defer cancelStorage()
	photoStorage, err := objectstorage.New(
		storageCtx,
		cfg.StorageEndpoint,
		cfg.StorageAccessKey,
		cfg.StorageSecretKey,
		cfg.StorageBucket,
		cfg.StorageUseSSL,
	)
	if err != nil {
		logger.Error("initialize photo storage", "error", err)
		os.Exit(1)
	}
	logger.Info("photo storage ready", "bucket", photoStorage.Bucket())
	photoService := service.NewPhotoService(photoRepository, photoStorage)
	photoHandler := handlers.NewPhotoHandler(photoService)

	redisClient, err := redis.New(cfg.RedisAddr)
	if err != nil {
		logger.Error("connect to Redis", "error", err)
		os.Exit(1)
	}
	defer redisClient.Close()

	publisher := events.NewPublisher(redisClient, cfg.RedisStream, cfg.PublisherPool, 256)
	publisher.Start(ctx, cfg.PublisherPool)
	defer publisher.Close()

	router := httptransport.NewRouter(httptransport.Deps{
		Publisher:      publisher,
		Authenticator:  authenticator,
		TokenManager:   tokenManager,
		Logger:         logger,
		AuthHandler:    authHandler,
		RibbonHandler:  ribbonHandler,
		ProfileHandler: profileHandler,
		ChatHandler:    chatHandler,
		PhotoHandler:   photoHandler,
	})

	srv := &http.Server{
		Addr:    ":" + cfg.HTTPPort,
		Handler: router,
	}

	go func() {
		logger.Info("tinder-core listening", "port", cfg.HTTPPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown", "error", err)
	}
}

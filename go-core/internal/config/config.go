package config

import "os"

type Config struct {
	Env           string
	HTTPPort      string
	PostgresDSN   string
	RedisAddr     string
	RedisStream   string
	PublisherPool int
}

func Load() Config {
	return Config{
		Env:           getEnv("APP_ENV", "local"),
		HTTPPort:      getEnv("HTTP_PORT", "8080"),
		PostgresDSN:   getEnv("POSTGRES_DSN", "postgres://tinder:tinder@localhost:5432/tinder?sslmode=disable"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisStream:   getEnv("REDIS_EVENTS_STREAM", "tinder:events"),
		PublisherPool: 4,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

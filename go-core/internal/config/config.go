package config

import "os"

type Config struct {
	Env              string
	HTTPPort         string
	PostgresDSN      string
	RedisAddr        string
	RedisStream      string
	PublisherPool    int
	JWTSigningKey    string
	JWTIssuer        string
	LogLevel         string
	StorageEndpoint  string
	StorageAccessKey string
	StorageSecretKey string
	StorageBucket    string
	StorageUseSSL    bool
}

func Load() Config {
	return Config{
		Env:              getEnv("APP_ENV", "local"),
		HTTPPort:         getEnv("HTTP_PORT", "8080"),
		PostgresDSN:      getEnv("POSTGRES_DSN", "postgres://tinder:tinder@localhost:5432/tinder?sslmode=disable"),
		RedisAddr:        getEnv("REDIS_ADDR", "localhost:6379"),
		RedisStream:      getEnv("REDIS_EVENTS_STREAM", "tinder:events"),
		PublisherPool:    4,
		JWTSigningKey:    os.Getenv("JWT_SIGNING_KEY"),
		JWTIssuer:        getEnv("JWT_ISSUER", "tinder-core"),
		LogLevel:         getEnv("LOG_LEVEL", "info"),
		StorageEndpoint:  getEnv("OBJECT_STORAGE_ENDPOINT", "localhost:9000"),
		StorageAccessKey: os.Getenv("OBJECT_STORAGE_ACCESS_KEY"),
		StorageSecretKey: os.Getenv("OBJECT_STORAGE_SECRET_KEY"),
		StorageBucket:    getEnv("OBJECT_STORAGE_BUCKET", "tinder-photos"),
		StorageUseSSL:    getEnv("OBJECT_STORAGE_USE_SSL", "false") == "true",
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

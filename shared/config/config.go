package config

import (
	"os"
	"time"
)

// Config holds all configuration for the application.
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Kafka    KafkaConfig
	JWT      JWTConfig
	SMTP     SMTPConfig
}

// ServerConfig holds server-specific configuration.
type ServerConfig struct {
	Port string
}

// DatabaseConfig holds database connection configuration.
type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	Host     string
	Port     string
	Password string
}

// KafkaConfig holds Kafka connection configuration.
type KafkaConfig struct {
	Brokers string
}

// JWTConfig holds JWT authentication configuration.
type JWTConfig struct {
	Secret        string
	AccessExpiry  time.Duration
	RefreshExpiry time.Duration
}

// SMTPConfig holds email server configuration.
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
}

// DSN returns the PostgreSQL connection string.
func (d DatabaseConfig) DSN() string {
	return "postgres://" + d.User + ":" + d.Password +
		"@" + d.Host + ":" + d.Port +
		"/" + d.DBName + "?sslmode=" + d.SSLMode
}

// Load reads configuration from environment variables with sensible defaults.
func Load() *Config {
	accessExpiry, _ := time.ParseDuration(getEnv("JWT_ACCESS_EXPIRY", "15m"))
	refreshExpiry, _ := time.ParseDuration(getEnv("JWT_REFRESH_EXPIRY", "168h"))

	return &Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnv("POSTGRES_PORT", "5432"),
			User:     getEnv("POSTGRES_USER", "entra"),
			Password: getEnv("POSTGRES_PASSWORD", "entra_secret"),
			DBName:   getEnv("POSTGRES_DB", "entra"),
			SSLMode:  getEnv("POSTGRES_SSLMODE", "disable"),
		},
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
		},
		Kafka: KafkaConfig{
			Brokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		},
		JWT: JWTConfig{
			Secret:        getEnv("JWT_SECRET", "change-me-to-a-strong-secret-key"),
			AccessExpiry:  accessExpiry,
			RefreshExpiry: refreshExpiry,
		},
		SMTP: SMTPConfig{
			Host:     getEnv("SMTP_HOST", "sandbox.smtp.mailtrap.io"),
			Port:     getEnv("SMTP_PORT", "2525"),
			Username: getEnv("SMTP_USER", ""),
			Password: getEnv("SMTP_PASS", ""),
		},
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

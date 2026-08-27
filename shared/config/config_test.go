package config_test

import (
	"os"
	"testing"
	"time"

	"entra-api/shared/config"
)

func TestConfig_Load_Defaults(t *testing.T) {
	// Clear any environment variables that might interfere
	envKeys := []string{
		"SERVER_PORT", "POSTGRES_HOST", "POSTGRES_PORT", "POSTGRES_USER",
		"POSTGRES_PASSWORD", "POSTGRES_DB", "POSTGRES_SSLMODE",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD",
		"KAFKA_BROKERS", "JWT_SECRET", "JWT_ACCESS_EXPIRY", "JWT_REFRESH_EXPIRY",
		"SMTP_HOST", "SMTP_PORT", "SMTP_USER", "SMTP_PASS",
	}
	for _, key := range envKeys {
		_ = os.Unsetenv(key)
	}

	cfg := config.Load()

	if cfg.Server.Port != "8080" {
		t.Errorf("expected default Server.Port 8080, got %s", cfg.Server.Port)
	}
	if cfg.Database.Host != "localhost" {
		t.Errorf("expected default Database.Host localhost, got %s", cfg.Database.Host)
	}
	if cfg.Database.Port != "5432" {
		t.Errorf("expected default Database.Port 5432, got %s", cfg.Database.Port)
	}
	if cfg.Database.User != "entra" {
		t.Errorf("expected default Database.User entra, got %s", cfg.Database.User)
	}
	if cfg.Database.Password != "entra_secret" {
		t.Errorf("expected default Database.Password entra_secret, got %s", cfg.Database.Password)
	}
	if cfg.Database.DBName != "entra" {
		t.Errorf("expected default Database.DBName entra, got %s", cfg.Database.DBName)
	}
	if cfg.Database.SSLMode != "disable" {
		t.Errorf("expected default Database.SSLMode disable, got %s", cfg.Database.SSLMode)
	}
	if cfg.Redis.Host != "localhost" {
		t.Errorf("expected default Redis.Host localhost, got %s", cfg.Redis.Host)
	}
	if cfg.Redis.Port != "6379" {
		t.Errorf("expected default Redis.Port 6379, got %s", cfg.Redis.Port)
	}
	if cfg.Kafka.Brokers != "localhost:9092" {
		t.Errorf("expected default Kafka.Brokers localhost:9092, got %s", cfg.Kafka.Brokers)
	}
	if cfg.JWT.Secret != "change-me-to-a-strong-secret-key" {
		t.Errorf("expected default JWT.Secret, got %s", cfg.JWT.Secret)
	}
	if cfg.JWT.AccessExpiry != 15*time.Minute {
		t.Errorf("expected default JWT.AccessExpiry 15m, got %v", cfg.JWT.AccessExpiry)
	}
	if cfg.JWT.RefreshExpiry != 168*time.Hour {
		t.Errorf("expected default JWT.RefreshExpiry 168h, got %v", cfg.JWT.RefreshExpiry)
	}
	if cfg.SMTP.Host != "sandbox.smtp.mailtrap.io" {
		t.Errorf("expected default SMTP.Host sandbox.smtp.mailtrap.io, got %s", cfg.SMTP.Host)
	}
	if cfg.SMTP.Port != "2525" {
		t.Errorf("expected default SMTP.Port 2525, got %s", cfg.SMTP.Port)
	}
}

func TestConfig_Load_CustomEnv(t *testing.T) {
	_ = os.Setenv("SERVER_PORT", "9090")
	_ = os.Setenv("POSTGRES_HOST", "db.internal")
	_ = os.Setenv("POSTGRES_PORT", "5433")
	_ = os.Setenv("POSTGRES_USER", "custom_user")
	_ = os.Setenv("POSTGRES_PASSWORD", "custom_pass")
	_ = os.Setenv("POSTGRES_DB", "entra_custom")
	_ = os.Setenv("POSTGRES_SSLMODE", "require")
	_ = os.Setenv("REDIS_HOST", "redis.internal")
	_ = os.Setenv("REDIS_PORT", "6380")
	_ = os.Setenv("REDIS_PASSWORD", "redis_secret")
	_ = os.Setenv("KAFKA_BROKERS", "kafka1:9092,kafka2:9092")
	_ = os.Setenv("JWT_SECRET", "custom-jwt-secret-999")
	_ = os.Setenv("JWT_ACCESS_EXPIRY", "30m")
	_ = os.Setenv("JWT_REFRESH_EXPIRY", "72h")
	_ = os.Setenv("SMTP_HOST", "smtp.sendgrid.net")
	_ = os.Setenv("SMTP_PORT", "587")
	_ = os.Setenv("SMTP_USER", "apikey")
	_ = os.Setenv("SMTP_PASS", "sg_secret")

	defer func() {
		os.Unsetenv("SERVER_PORT")
		os.Unsetenv("POSTGRES_HOST")
		os.Unsetenv("POSTGRES_PORT")
		os.Unsetenv("POSTGRES_USER")
		os.Unsetenv("POSTGRES_PASSWORD")
		os.Unsetenv("POSTGRES_DB")
		os.Unsetenv("POSTGRES_SSLMODE")
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASSWORD")
		os.Unsetenv("KAFKA_BROKERS")
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("JWT_ACCESS_EXPIRY")
		os.Unsetenv("JWT_REFRESH_EXPIRY")
		os.Unsetenv("SMTP_HOST")
		os.Unsetenv("SMTP_PORT")
		os.Unsetenv("SMTP_USER")
		os.Unsetenv("SMTP_PASS")
	}()

	cfg := config.Load()

	if cfg.Server.Port != "9090" {
		t.Errorf("expected Server.Port 9090, got %s", cfg.Server.Port)
	}
	if cfg.Database.Host != "db.internal" {
		t.Errorf("expected Database.Host db.internal, got %s", cfg.Database.Host)
	}
	if cfg.Database.DBName != "entra_custom" {
		t.Errorf("expected Database.DBName entra_custom, got %s", cfg.Database.DBName)
	}
	if cfg.Database.SSLMode != "require" {
		t.Errorf("expected Database.SSLMode require, got %s", cfg.Database.SSLMode)
	}
	if cfg.Redis.Host != "redis.internal" {
		t.Errorf("expected Redis.Host redis.internal, got %s", cfg.Redis.Host)
	}
	if cfg.Redis.Password != "redis_secret" {
		t.Errorf("expected Redis.Password redis_secret, got %s", cfg.Redis.Password)
	}
	if cfg.Kafka.Brokers != "kafka1:9092,kafka2:9092" {
		t.Errorf("expected Kafka.Brokers kafka1:9092,kafka2:9092, got %s", cfg.Kafka.Brokers)
	}
	if cfg.JWT.Secret != "custom-jwt-secret-999" {
		t.Errorf("expected JWT.Secret custom-jwt-secret-999, got %s", cfg.JWT.Secret)
	}
	if cfg.JWT.AccessExpiry != 30*time.Minute {
		t.Errorf("expected JWT.AccessExpiry 30m, got %v", cfg.JWT.AccessExpiry)
	}
	if cfg.JWT.RefreshExpiry != 72*time.Hour {
		t.Errorf("expected JWT.RefreshExpiry 72h, got %v", cfg.JWT.RefreshExpiry)
	}
	if cfg.SMTP.Host != "smtp.sendgrid.net" {
		t.Errorf("expected SMTP.Host smtp.sendgrid.net, got %s", cfg.SMTP.Host)
	}
}

func TestDatabaseConfig_DSN(t *testing.T) {
	dbCfg := config.DatabaseConfig{
		Host:     "127.0.0.1",
		Port:     "5432",
		User:     "entra_app",
		Password: "strong_password",
		DBName:   "entra_auth",
		SSLMode:  "disable",
	}

	expectedDSN := "postgres://entra_app:strong_password@127.0.0.1:5432/entra_auth?sslmode=disable"
	if actualDSN := dbCfg.DSN(); actualDSN != expectedDSN {
		t.Errorf("expected DSN %s, got %s", expectedDSN, actualDSN)
	}
}

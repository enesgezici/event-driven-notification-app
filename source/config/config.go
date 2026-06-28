package config

import (
	"os"
)

type Config struct {
	ServerAddress string
	DatabaseURL   string
	WebhookURL    string
}

func LoadConfig() *Config {
	addr := os.Getenv("SERVER_ADDRESS")
	if addr == "" {
		addr = ":8080"
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://notification:notification@localhost:5432/notifications?sslmode=disable"
	}
	webhook := os.Getenv("WEBHOOK_URL")
	return &Config{
		ServerAddress: addr,
		DatabaseURL:   databaseURL,
		WebhookURL:    webhook,
	}
}

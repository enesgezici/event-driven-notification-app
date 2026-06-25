package config

import (
	"log"
	"os"
)

type Config struct {
	ServerAddress string
	DatabasePath  string
	WebhookURL    string
}

func LoadConfig() *Config {
	addr := os.Getenv("SERVER_ADDRESS")
	if addr == "" {
		addr = ":8080"
	}
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "./data/notifications.db"
	}
	webhook := os.Getenv("WEBHOOK_URL")
	if webhook == "" {
		log.Fatal("WEBHOOK_URL environment variable is required")
	}
	return &Config{
		ServerAddress: addr,
		DatabasePath:  path,
		WebhookURL:    webhook,
	}
}

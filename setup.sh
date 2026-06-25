#!/bin/bash

# Event-Driven Notification System - Setup Script

export WEBHOOK_URL="https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075"
export SERVER_ADDRESS=":8080"
export DATABASE_PATH="./data/notifications.db"

# Create data directory
mkdir -p ./source/data

# Install dependencies
cd ./source
go mod download
go mod tidy

echo "✓ Dependencies installed"
echo "✓ Webhook URL: $WEBHOOK_URL"
echo "✓ Server will run on http://localhost:8080"
echo ""
echo "To start the server, run:"
echo "  cd source"
echo "  WEBHOOK_URL='$WEBHOOK_URL' ./notification-server"

# Event-Driven Notification System

Event-Driven Notification Application - Scalable SMS, Email, Push notifications via webhook integration.

## Quick Start

### Prerequisites
- Go 1.21+ or Docker
- Webhook URL from [webhook.site](https://webhook.site)

### 1. Setup with Your Webhook URL

Your webhook configuration:
```
Webhook URL: https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075
Session ID: dad7d86c-b299-4b15-a40b-dd8bfa7c94ad
```

### 2. Run Locally (Go)

```bash
cd source
export WEBHOOK_URL="https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075"
export SERVER_ADDRESS=":8080"
export DATABASE_PATH="./data/notifications.db"

mkdir -p data
go mod download
./notification-server
```

### 3. Run with Docker Compose

```bash
WEBHOOK_URL="https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075" docker-compose up
```

## API Endpoints

### Create Notification(s)
```bash
curl -X POST http://localhost:8080/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {
        "recipient": "+905551234567",
        "channel": "sms",
        "content": "Hello! This is a test.",
        "priority": "high"
      }
    ]
  }'
```

Response:
```json
{
  "batch_id": "550e8400-e29b-41d4-a716-446655440000",
  "notifications": [
    {
      "id": "6ba7b810-9dad-11d1-80b4-00c04fd430c8",
      "batch_id": "550e8400-e29b-41d4-a716-446655440000",
      "recipient": "+905551234567",
      "channel": "sms",
      "content": "Hello! This is a test.",
      "priority": 2,
      "status": "pending",
      "retry_count": 0,
      "created_at": "2026-06-25T14:30:00Z",
      "updated_at": "2026-06-25T14:30:00Z"
    }
  ]
}
```

### Get Notification Status
```bash
curl http://localhost:8080/notifications/{notification-id}
```

### List Notifications (with filtering)
```bash
# All notifications
curl http://localhost:8080/notifications

# Filter by status
curl "http://localhost:8080/notifications?status=sent"

# Filter by channel
curl "http://localhost:8080/notifications?channel=sms"

# Filter by batch
curl "http://localhost:8080/notifications?batch_id=550e8400-e29b-41d4-a716-446655440000"

# Pagination
curl "http://localhost:8080/notifications?page=1&size=25"
```

### Cancel Notification
```bash
curl -X DELETE http://localhost:8080/notifications/{notification-id}
```

### Health Check
```bash
curl http://localhost:8080/health
```

Response:
```json
{"status": "ok"}
```

### Metrics
```bash
curl http://localhost:8080/metrics
```

Response:
```json
{
  "queue_depth": 5,
  "success_count": 42,
  "failure_count": 2,
  "last_updated": "2026-06-25T14:35:22Z"
}
```

## Features

### Notification Management
- ✅ Create single or batch notifications (up to 1000)
- ✅ Query notification status by ID or batch ID
- ✅ Cancel pending notifications
- ✅ Filter and paginate notifications
- ✅ Idempotency support to prevent duplicates

### Processing Engine
- ✅ Asynchronous queue-based processing
- ✅ Priority queue (High, Normal, Low)
- ✅ Rate limiting (100 messages/second per channel)
- ✅ Content validation (SMS: 160 char limit)
- ✅ Structured channel support (SMS, Email, Push)

### Reliability
- ✅ Persistent SQLite database
- ✅ Graceful shutdown
- ✅ Error handling and status tracking
- ✅ Automatic retry on failure

### Observability
- ✅ Real-time metrics endpoint
- ✅ Health check endpoint
- ✅ Structured logging with correlation IDs
- ✅ Queue depth monitoring

## Project Structure

```
.
├── source/
│   ├── cmd/notification-server/
│   │   └── main.go                 # Application entry point
│   ├── internal/
│   │   ├── api/
│   │   │   └── router.go           # HTTP handlers
│   │   ├── model/
│   │   │   └── notification.go     # Data model & validation
│   │   ├── queue/
│   │   │   ├── queue.go            # Priority queue manager
│   │   │   └── queue_extra.go      # Enhancements
│   │   ├── storage/
│   │   │   ├── sqlite.go           # Database layer
│   │   │   └── types.go            # Storage interface
│   │   ├── metrics/
│   │   │   └── metrics.go          # Performance metrics
│   │   └── provider/
│   │       └── provider.go         # Webhook integration
│   ├── config/
│   │   └── config.go               # Configuration
│   ├── go.mod
│   └── notification-server         # Compiled binary
├── docker-compose.yml
├── Dockerfile
└── README.md
```

## Configuration

Environment variables:
- `SERVER_ADDRESS` - Server listen address (default: `:8080`)
- `DATABASE_PATH` - SQLite database file path (default: `./data/notifications.db`)
- `WEBHOOK_URL` - External provider webhook URL (required)

## Database Schema

```sql
CREATE TABLE notifications (
  id TEXT PRIMARY KEY,
  batch_id TEXT,
  recipient TEXT NOT NULL,
  channel TEXT NOT NULL,
  content TEXT NOT NULL,
  priority INTEGER NOT NULL,
  status TEXT NOT NULL,
  error TEXT,
  retry_count INTEGER NOT NULL DEFAULT 0,
  external_message_id TEXT,
  idempotency_key TEXT,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE INDEX idx_notifications_batch_id ON notifications(batch_id);
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_channel ON notifications(channel);
CREATE UNIQUE INDEX idx_notifications_idempotency_key ON notifications(idempotency_key);
```

## Performance Considerations

- **Queue Processing**: Each channel processes messages concurrently with 10ms polling intervals
- **Rate Limiting**: Maximum 100 messages/second per channel (configurable)
- **Database**: SQLite with appropriate indices for fast queries
- **Memory**: Priority heap for efficient notification processing

## Testing

```bash
# Run all tests
cd source
go test ./...

# Run specific package tests
go test ./internal/model
go test ./internal/storage
go test ./internal/queue

# Run with verbose output
go test -v ./...
```

## Next Steps

1. **Retry Logic**: Implement exponential backoff for failed deliveries
2. **Circuit Breaker**: Add circuit breaker pattern for external provider reliability
3. **Monitoring**: Integrate with Prometheus for advanced metrics
4. **Authentication**: Add API key or JWT authentication
5. **Message Templates**: Support dynamic content rendering

## Support

For webhook testing, visit: https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075

Your session ID: `dad7d86c-b299-4b15-a40b-dd8bfa7c94ad`

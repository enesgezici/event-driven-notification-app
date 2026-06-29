# Event-Driven Notification System

Event-Driven Notification Application - Scalable SMS, Email, Push notifications via webhook integration.

## Quick Start

### Prerequisites
- Go 1.25+ or Docker
- PostgreSQL 16+ for local runs, or Docker Compose
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
export DATABASE_URL="postgres://notification:notification@localhost:5432/notifications?sslmode=disable"

go mod download
go build -o notification-server ./cmd/notification-server
./notification-server
```

### 3. Run with Docker Compose

```bash
WEBHOOK_URL="https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075" docker-compose up
```

## API Endpoints

OpenAPI specification is available at `docs/openapi.yaml`.

### Create Notification(s)
```bash
curl -X POST http://localhost:8080/notifications \
  -H "Content-Type: application/json" \
  -H "X-Correlation-ID: request-123" \
  -H "Idempotency-Key: campaign-2026-06-25-001" \
  -d '{
    "notifications": [
      {
        "recipient": "+905551234567",
        "channel": "sms",
        "content": "Hello! This is a test.",
        "priority": "high",
        "scheduled_at": "2026-07-01T15:30:00Z"
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

# Filter by date range (RFC3339)
curl "http://localhost:8080/notifications?from=2026-06-25T00:00:00Z&to=2026-06-26T00:00:00Z"

# Pagination
curl "http://localhost:8080/notifications?page=1&size=25"
```

### Query Batch Status
```bash
curl http://localhost:8080/batches/{batch-id}/notifications
```

### Create Template
```bash
curl -X POST http://localhost:8080/templates \
  -H "Content-Type: application/json" \
  -d '{
    "name": "welcome-sms",
    "channel": "sms",
    "content": "Hello {{.first_name}}, your code is {{.code}}"
  }'
```

### Send With Template
```bash
curl -X POST http://localhost:8080/notifications \
  -H "Content-Type: application/json" \
  -d '{
    "notifications": [
      {
        "recipient": "+905551234567",
        "channel": "sms",
        "template_id": "{template-id}",
        "template_data": {
          "first_name": "Enes",
          "code": "123456"
        },
        "priority": "normal"
      }
    ]
  }'
```

### List Templates
```bash
curl http://localhost:8080/templates
```

### Cancel Notification
```bash
curl -X DELETE http://localhost:8080/notifications/{notification-id}
```

Returns `409 Conflict` when the notification does not exist or is already in a non-cancellable state.

### Health Check
```bash
curl http://localhost:8080/health
```

Response:
```json
{"status": "ok"}
```

Returns `503 Service Unavailable` when PostgreSQL is not reachable.

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
  "retry_count": 3,
  "average_latency_ms": 84.2,
  "success_rate": 0.95,
  "failure_rate": 0.05,
  "total_deliveries": 44,
  "last_updated": "2026-06-25T14:35:22Z"
}
```

## Features

### Notification Management
- ✅ Create single or batch notifications (up to 1000)
- ✅ Query notification status by ID or batch ID
- ✅ Cancel pending notifications
- ✅ Filter and paginate notifications
- ✅ Atomic batch creation with transaction rollback on partial failure
- ✅ Database-backed idempotency support to prevent duplicate sends under concurrent requests
- ✅ Scheduled notifications with future `scheduled_at` delivery
- ✅ Template system with variable substitution

### Processing Engine
- ✅ Asynchronous queue-based processing
- ✅ Priority queue (High, Normal, Low)
- ✅ PostgreSQL-backed atomic job claiming for multi-instance workers
- ✅ Per-channel dispatch pacing (100 attempts/second per running instance)
- ✅ Content validation (SMS: 160 char limit)
- ✅ Structured channel support (SMS, Email, Push)

### Reliability
- ✅ Persistent PostgreSQL database
- ✅ Graceful shutdown
- ✅ Error handling and status tracking
- ✅ Automatic retry on failure with exponential backoff
- ✅ Stale queued job recovery after worker crashes
- ✅ PostgreSQL persistence with connection pooling and query indexes

### Observability
- ✅ Real-time metrics endpoint
- ✅ Health check endpoint
- ✅ JSON structured logging with correlation IDs
- ✅ PostgreSQL-backed queue depth monitoring
- ✅ GitHub Actions CI for formatting and tests

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
│   │   │   ├── postgres.go         # PostgreSQL database layer
│   │   │   └── types.go            # Storage interface
│   │   ├── metrics/
│   │   │   └── metrics.go          # Performance metrics
│   │   └── provider/
│   │       └── provider.go         # Webhook integration
│   ├── config/
│   │   └── config.go               # Configuration
│   ├── migrations/
│   │   └── 001_create_notifications.sql
│   └── go.mod
├── docker-compose.yml
├── Dockerfile
└── README.md
```

## Configuration

Environment variables:
- `SERVER_ADDRESS` - Server listen address (default: `:8080`)
- `DATABASE_URL` - PostgreSQL connection string (default: `postgres://notification:notification@localhost:5432/notifications?sslmode=disable`)
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
  template_id TEXT,
  template_data JSONB,
  scheduled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE idempotency_keys (
  idempotency_key TEXT PRIMARY KEY,
  batch_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE templates (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  channel TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_notifications_batch_id ON notifications(batch_id);
CREATE INDEX idx_notifications_status ON notifications(status);
CREATE INDEX idx_notifications_channel ON notifications(channel);
CREATE INDEX idx_notifications_created_at ON notifications(created_at DESC);
CREATE INDEX idx_notifications_scheduled_at ON notifications(scheduled_at);
CREATE INDEX idx_notifications_status_channel_created_at ON notifications(status, channel, created_at DESC);
CREATE INDEX idx_notifications_claim_due ON notifications(status, channel, priority, scheduled_at, created_at);
CREATE INDEX idx_notifications_stale_queued ON notifications(status, updated_at);
CREATE INDEX idx_notifications_idempotency_key ON notifications(idempotency_key);
CREATE INDEX idx_templates_channel ON templates(channel);
```

## Performance Considerations

- **Queue Processing**: Each channel processes messages concurrently with 10ms polling intervals
- **Dispatch pacing**: Each channel worker ticks every 10ms, for 100 dispatch attempts/second per channel per running instance
- **Worker coordination**: Workers atomically claim due notifications in PostgreSQL to avoid duplicate sends across instances
- **Database**: PostgreSQL with pooled connections and indexed status/channel/date queries
- **Consistency**: Batch inserts are transactional, and idempotency keys are protected by a PostgreSQL primary key
- **Recovery**: Stale queued notifications are eligible for re-claiming after a bounded timeout
- **Metrics**: Queue depth is read from PostgreSQL pending/queued notification counts

## Testing

```bash
# Build
cd source
go build -o notification-server ./cmd/notification-server

# Run all tests
go test ./...

# Run specific package tests
go test ./internal/model
go test ./internal/queue
go test ./internal/api

# Run with verbose output
go test -v ./...
```

## Next Steps

1. **Circuit Breaker**: Add circuit breaker pattern for external provider reliability
2. **Monitoring**: Integrate with Prometheus for advanced metrics
3. **Authentication**: Add API key or JWT authentication
4. **WebSocket Updates**: Stream status changes to API consumers
5. **Distributed Tracing**: Add OpenTelemetry spans across API, queue and provider calls

## Support

For webhook testing, visit: https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075

Your session ID: `dad7d86c-b299-4b15-a40b-dd8bfa7c94ad`

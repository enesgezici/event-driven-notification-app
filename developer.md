# Event-Driven Notification System

## Project Overview

This project provides a scalable notification system capable of sending notifications through SMS, Email, and Push channels. The system receives notifications, stores them in the database, processes them in a queue based on priority, and delivers them to an external provider via webhook integration.

## Components

### 1. API Server

* `source/cmd/notification-server/main.go`

  * Server startup and graceful shutdown for HTTP requests and worker loops
  * Configuration loading
  * Router initialization

### 2. Configuration

* `source/config/config.go`

  * Environment variables: `SERVER_ADDRESS`, `DATABASE_URL`, `WEBHOOK_URL`
  * Mandatory webhook URL validation

### 3. Data Model

* `source/internal/model/notification.go`

  * Notification status and priority constants
  * Notification structure (`Notification`)
  * Channel and content validations

### 4. Storage Layer

* `source/internal/storage/postgres.go`

  * PostgreSQL connection handling
  * Schema migrations
  * Transaction-based batch notification creation, updates, queries, and cancellations
  * Filtering and pagination support

### 5. Queue Management

* `source/internal/queue/queue.go`

  * Priority queue implementation
  * Channel-based worker loops
  * PostgreSQL-backed atomic job claiming
  * Notification processing and delivery logic
  * Queue depth monitoring

### 6. Provider Integration

* `source/internal/provider/provider.go`

  * Sends POST requests to external providers such as webhook.site
  * Accepts both `200 OK` and `202 Accepted` responses
  * Generates UUIDs when the provider does not return a JSON response

### 7. Metrics and Observability

* `source/internal/metrics/metrics.go`

  * Queue depth metrics
  * Success and failure counters
  * Last update timestamps
  * JSON-based metrics output

### 8. API Routing

* `source/internal/api/router.go`

  * Create notification: `POST /notifications`
  * Retrieve notification status: `GET /notifications/{id}`
  * List notifications with filtering: `GET /notifications`
  * Cancel notification: `DELETE /notifications/{id}`
  * Health check with PostgreSQL dependency check: `GET /health`
  * Metrics endpoint: `GET /metrics`

## Usage

1. Navigate to the `source` directory.
2. Set the required environment variables:

```bash
export WEBHOOK_URL="https://webhook.site/fa8d1250-1966-4b1a-91f5-4c2847138075"
export SERVER_ADDRESS=":8080"
export DATABASE_URL="postgres://notification:notification@localhost:5432/notifications?sslmode=disable"
```

3. Build and run the server:

```bash
go build -o notification-server ./cmd/notification-server
./notification-server
```

## Features

* Bulk notification creation (up to 1000 notifications per request)
* Filtering and pagination support
* Channel-based priority processing
* Idempotency support protected by PostgreSQL primary keys
* Scheduled notification delivery (`scheduled_at`)
* Template system with variable interpolation
* Real-time metrics
* Health check endpoint
* External webhook integration

## Development Notes

* `docker-compose.yml` and `Dockerfile` are included.
* `README.md` and `IMPLEMENTATION.md` provide project documentation.
* `test.sh` simplifies basic end-to-end testing against a running server.
* PostgreSQL is used for persistent storage and is better suited than SQLite for high traffic and concurrent writes.
* PostgreSQL-backed claiming allows multiple application instances to share work; for larger sustained traffic, Redis, RabbitMQ, Kafka, or another dedicated queue can be integrated.
* Logs are generated in JSON structured format and include a correlation ID field where request context is available.

## Important Considerations

* webhook.site may return `200 OK` responses by default, so the provider accepts both `200 OK` and `202 Accepted` responses.
* If the `idempotency_key` field is empty, it is stored as `NULL`. When provided, the `idempotency_keys` table atomically prevents duplicate batch requests.
* Email deliveries are not actually sent to external recipients; delivery is simulated using webhook.site.

## Future Improvements

* Add a dead-letter queue table
* Use Redis or RabbitMQ for persistent queue management
* Add API authentication and authorization
* Extend monitoring with Prometheus and Grafana integration
* Add WebSocket-based status updates and distributed tracing

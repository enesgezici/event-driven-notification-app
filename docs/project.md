# Project Notes

This project is a single-instance event-driven notification service written in Go. It accepts batch notification requests, persists them in PostgreSQL, queues them by priority, and dispatches them to an external webhook provider.

## Runtime Components

- `source/cmd/notification-server/main.go`: Loads configuration, opens PostgreSQL, runs migrations, starts workers, and manages shutdown.
- `source/internal/api/router.go`: HTTP endpoints for notifications, batches, templates, health, and metrics.
- `source/internal/storage/postgres.go`: PostgreSQL persistence, migrations, filters, idempotency keys, and cancellation updates.
- `source/internal/queue/queue.go`: In-memory priority queue, per-channel workers, scheduled delivery, retries, and worker shutdown.
- `source/internal/provider/provider.go`: Webhook delivery client.
- `source/internal/metrics/metrics.go`: Queue depth, delivery counters, retry counts, and latency metrics.

## Current Guarantees

- PostgreSQL-backed notification persistence.
- Atomic batch inserts with idempotency-key protection.
- Priority ordering within each channel.
- Future `scheduled_at` delivery.
- Retry with exponential backoff up to three attempts.
- HTTP shutdown plus worker cancellation on process termination.

## Known Boundaries

- Queue state is in memory, so horizontal scaling requires Redis, RabbitMQ, or another shared queue.
- Metrics are process-local and reset on restart.
- External delivery is simulated through webhook-style HTTP delivery.
- Authentication, authorization, tenant isolation, and dead-letter persistence are future work.

## Verification

Run unit tests from `source`:

```bash
go test ./...
go vet ./...
test -z "$(gofmt -l .)"
```

For manual end-to-end checks, start PostgreSQL and the server, then run:

```bash
bash ../test.sh
```

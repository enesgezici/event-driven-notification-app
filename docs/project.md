# Project Notes

This project is an event-driven notification service written in Go. It accepts batch notification requests, persists them in PostgreSQL, atomically claims due work from the database, and dispatches notifications to an external webhook provider.

## Runtime Components

- `source/cmd/notification-server/main.go`: Loads configuration, opens PostgreSQL, runs migrations, starts workers, and manages shutdown.
- `source/internal/api/router.go`: HTTP endpoints for notifications, batches, templates, health, and metrics.
- `source/internal/storage/postgres.go`: PostgreSQL persistence, migrations, filters, idempotency keys, atomic job claiming, and cancellation updates.
- `source/internal/queue/queue.go`: Priority queue fast path, PostgreSQL-backed worker claiming, per-channel dispatch pacing, scheduled delivery, retries, and worker shutdown.
- `source/internal/provider/provider.go`: Webhook delivery client.
- `source/internal/metrics/metrics.go`: Database-backed queue depth, delivery counters, retry counts, and latency metrics.

## Current Guarantees

- PostgreSQL-backed notification persistence and atomic work claiming.
- Atomic batch inserts with idempotency-key protection.
- Priority ordering within each channel.
- Future `scheduled_at` delivery.
- Retry with exponential backoff up to three attempts.
- Stale queued job recovery after worker crashes.
- Health checks verify PostgreSQL availability.
- HTTP shutdown plus worker cancellation on process termination.

## Known Boundaries

- PostgreSQL-backed claiming allows multiple API instances to share work, but very high-volume deployments may still prefer Redis, RabbitMQ, Kafka, or another dedicated queue.
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

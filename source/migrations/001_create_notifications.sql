CREATE TABLE IF NOT EXISTS notifications (
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
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS idempotency_keys (
  idempotency_key TEXT PRIMARY KEY,
  batch_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

INSERT INTO idempotency_keys (idempotency_key, batch_id, created_at)
SELECT idempotency_key, MIN(batch_id), MIN(created_at)
FROM notifications
WHERE idempotency_key IS NOT NULL
GROUP BY idempotency_key
ON CONFLICT (idempotency_key) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_notifications_batch_id ON notifications(batch_id);
CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
CREATE INDEX IF NOT EXISTS idx_notifications_channel ON notifications(channel);
CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_status_channel_created_at ON notifications(status, channel, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_idempotency_key ON notifications(idempotency_key);

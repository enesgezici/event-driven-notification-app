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
  template_id TEXT,
  template_data JSONB,
  scheduled_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

ALTER TABLE notifications ADD COLUMN IF NOT EXISTS template_id TEXT;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS template_data JSONB;
ALTER TABLE notifications ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ;

CREATE TABLE IF NOT EXISTS idempotency_keys (
  idempotency_key TEXT PRIMARY KEY,
  batch_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS templates (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  channel TEXT NOT NULL,
  content TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
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
CREATE INDEX IF NOT EXISTS idx_notifications_scheduled_at ON notifications(scheduled_at);
CREATE INDEX IF NOT EXISTS idx_notifications_status_channel_created_at ON notifications(status, channel, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_notifications_claim_due ON notifications(status, channel, priority, scheduled_at, created_at);
CREATE INDEX IF NOT EXISTS idx_notifications_stale_queued ON notifications(status, updated_at);
CREATE INDEX IF NOT EXISTS idx_notifications_idempotency_key ON notifications(idempotency_key);
CREATE INDEX IF NOT EXISTS idx_templates_channel ON templates(channel);

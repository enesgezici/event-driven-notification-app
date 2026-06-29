package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "github.com/lib/pq"
	"github.com/yourusername/event-driven-notification-app/internal/model"
)

type PostgresStorage struct {
	db *sql.DB
}

const staleQueuedAfter = 2 * time.Minute

func NewPostgresStorage(databaseURL string) (*PostgresStorage, error) {
	db, err := sql.Open("postgres", databaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(30 * time.Minute)
	db.SetConnMaxIdleTime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &PostgresStorage{db: db}, nil
}

func (s *PostgresStorage) Close() error {
	return s.db.Close()
}

func (s *PostgresStorage) Ping() error {
	return s.db.Ping()
}

func (s *PostgresStorage) Migrate() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS notifications (
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
		);`,
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS template_id TEXT;`,
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS template_data JSONB;`,
		`ALTER TABLE notifications ADD COLUMN IF NOT EXISTS scheduled_at TIMESTAMPTZ;`,
		`CREATE TABLE IF NOT EXISTS idempotency_keys (
			idempotency_key TEXT PRIMARY KEY,
			batch_id TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS templates (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			channel TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);`,
		`INSERT INTO idempotency_keys (idempotency_key, batch_id, created_at)
		 SELECT idempotency_key, MIN(batch_id), MIN(created_at)
		 FROM notifications
		 WHERE idempotency_key IS NOT NULL
		 GROUP BY idempotency_key
		 ON CONFLICT (idempotency_key) DO NOTHING;`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_batch_id ON notifications(batch_id);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_channel ON notifications(channel);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_created_at ON notifications(created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_scheduled_at ON notifications(scheduled_at);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_status_channel_created_at ON notifications(status, channel, created_at DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_claim_due ON notifications(status, channel, priority, scheduled_at, created_at);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_stale_queued ON notifications(status, updated_at);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_idempotency_key ON notifications(idempotency_key);`,
		`CREATE INDEX IF NOT EXISTS idx_templates_channel ON templates(channel);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

func (s *PostgresStorage) SaveNotification(n *model.Notification) error {
	templateData, err := marshalTemplateData(n.TemplateData)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(insertNotificationQuery,
		n.ID,
		n.BatchID,
		n.Recipient,
		n.Channel,
		n.Content,
		n.Priority,
		n.Status,
		n.Error,
		n.RetryCount,
		n.ExternalMessageID,
		n.IdempotencyKey,
		n.TemplateID,
		templateData,
		nullableTime(n.ScheduledAt),
		n.CreatedAt.UTC(),
		n.UpdatedAt.UTC(),
	)
	return err
}

func (s *PostgresStorage) SaveNotificationsBatch(idempotencyKey string, notifications []*model.Notification) (bool, []*model.Notification, error) {
	if len(notifications) == 0 {
		return true, nil, nil
	}

	tx, err := s.db.Begin()
	if err != nil {
		return false, nil, err
	}
	defer tx.Rollback()

	if idempotencyKey != "" {
		result, err := tx.Exec(
			`INSERT INTO idempotency_keys (idempotency_key, batch_id, created_at) VALUES ($1, $2, $3) ON CONFLICT (idempotency_key) DO NOTHING`,
			idempotencyKey,
			notifications[0].BatchID,
			time.Now().UTC(),
		)
		if err != nil {
			return false, nil, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return false, nil, err
		}
		if affected == 0 {
			existing, err := s.getNotificationsByIdempotencyKeyTx(tx, idempotencyKey)
			if err != nil {
				return false, nil, err
			}
			return false, existing, tx.Commit()
		}
	}

	stmt, err := tx.Prepare(insertNotificationQuery)
	if err != nil {
		return false, nil, err
	}
	defer stmt.Close()

	for _, n := range notifications {
		templateData, err := marshalTemplateData(n.TemplateData)
		if err != nil {
			return false, nil, err
		}
		if _, err := stmt.Exec(
			n.ID,
			n.BatchID,
			n.Recipient,
			n.Channel,
			n.Content,
			n.Priority,
			n.Status,
			n.Error,
			n.RetryCount,
			n.ExternalMessageID,
			n.IdempotencyKey,
			n.TemplateID,
			templateData,
			nullableTime(n.ScheduledAt),
			n.CreatedAt.UTC(),
			n.UpdatedAt.UTC(),
		); err != nil {
			return false, nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return false, nil, err
	}
	return true, notifications, nil
}

const notificationSelectColumns = `id, batch_id, recipient, channel, content, priority, status, error, retry_count, external_message_id, idempotency_key, template_id, template_data, scheduled_at, created_at, updated_at`

const insertNotificationQuery = `INSERT INTO notifications (id, batch_id, recipient, channel, content, priority, status, error, retry_count, external_message_id, idempotency_key, template_id, template_data, scheduled_at, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NULLIF($11, ''), NULLIF($12, ''), $13::jsonb, $14, $15, $16)`

func (s *PostgresStorage) GetNotificationByID(id string) (*model.Notification, error) {
	row := s.db.QueryRow(`SELECT `+notificationSelectColumns+` FROM notifications WHERE id = $1`, id)
	return scanNotification(row)
}

func (s *PostgresStorage) UpdateNotification(n *model.Notification) error {
	_, err := s.db.Exec(
		`UPDATE notifications SET status = $1, error = $2, retry_count = $3, external_message_id = $4, scheduled_at = $5, updated_at = $6 WHERE id = $7`,
		n.Status,
		n.Error,
		n.RetryCount,
		n.ExternalMessageID,
		nullableTime(n.ScheduledAt),
		n.UpdatedAt.UTC(),
		n.ID,
	)
	return err
}

func (s *PostgresStorage) ClaimNotification(id string) (*model.Notification, bool, error) {
	row := s.db.QueryRow(
		`UPDATE notifications
		 SET status = $1, updated_at = $2
		 WHERE id = $3
		   AND status = $4
		   AND (scheduled_at IS NULL OR scheduled_at <= $2)
		 RETURNING `+notificationSelectColumns,
		model.StatusQueued,
		time.Now().UTC(),
		id,
		model.StatusPending,
	)
	n, err := scanNotification(row)
	if err == sql.ErrNoRows {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return n, true, nil
}

func (s *PostgresStorage) ClaimNextDueNotification(channel string) (*model.Notification, bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, false, err
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	staleBefore := now.Add(-staleQueuedAfter)
	var id string
	err = tx.QueryRow(
		`SELECT id
		 FROM notifications
		 WHERE channel = $1
		   AND (
		     (status = $2 AND (scheduled_at IS NULL OR scheduled_at <= $3))
		     OR (status = $4 AND updated_at < $5)
		   )
		 ORDER BY priority ASC, COALESCE(scheduled_at, created_at) ASC
		 FOR UPDATE SKIP LOCKED
		 LIMIT 1`,
		channel,
		model.StatusPending,
		now,
		model.StatusQueued,
		staleBefore,
	).Scan(&id)
	if err == sql.ErrNoRows {
		return nil, false, tx.Commit()
	}
	if err != nil {
		return nil, false, err
	}

	row := tx.QueryRow(
		`UPDATE notifications
		 SET status = $1, updated_at = $2
		 WHERE id = $3
		 RETURNING `+notificationSelectColumns,
		model.StatusQueued,
		now,
		id,
	)
	n, err := scanNotification(row)
	if err != nil {
		return nil, false, err
	}
	if err := tx.Commit(); err != nil {
		return nil, false, err
	}
	return n, true, nil
}

func (s *PostgresStorage) ListNotifications(filters map[string]string, page, size int) ([]*model.Notification, error) {
	query := `SELECT ` + notificationSelectColumns + ` FROM notifications`
	clauses := []string{}
	args := []any{}

	addFilter := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}

	if v := filters["batch_id"]; v != "" {
		addFilter(`batch_id = $%d`, v)
	}
	if v := filters["status"]; v != "" {
		addFilter(`status = $%d`, v)
	}
	if v := filters["channel"]; v != "" {
		addFilter(`channel = $%d`, v)
	}
	if v := filters["recipient"]; v != "" {
		addFilter(`recipient = $%d`, v)
	}
	if v := filters["from"]; v != "" {
		addFilter(`created_at >= $%d::timestamptz`, v)
	}
	if v := filters["to"]; v != "" {
		addFilter(`created_at <= $%d::timestamptz`, v)
	}

	if len(clauses) > 0 {
		query += " WHERE " + joinClauses(clauses, " AND ")
	}
	args = append(args, size, (page-1)*size)
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", len(args)-1, len(args))

	return s.queryNotifications(query, args...)
}

func (s *PostgresStorage) GetPendingNotifications() ([]*model.Notification, error) {
	query := `SELECT ` + notificationSelectColumns + `
		FROM notifications
		WHERE status = $1
		  AND (scheduled_at IS NULL OR scheduled_at <= $2)
		ORDER BY priority ASC, COALESCE(scheduled_at, created_at) ASC`
	return s.queryNotifications(query, model.StatusPending, time.Now().UTC())
}

func (s *PostgresStorage) GetPendingNotificationsByBatch(batchID string) ([]*model.Notification, error) {
	query := `SELECT ` + notificationSelectColumns + ` FROM notifications WHERE batch_id = $1 ORDER BY priority ASC, COALESCE(scheduled_at, created_at) ASC`
	return s.queryNotifications(query, batchID)
}

func (s *PostgresStorage) GetNotificationsByIdempotencyKey(key string) ([]*model.Notification, error) {
	query := `SELECT ` + notificationSelectColumns + ` FROM notifications WHERE idempotency_key = $1`
	return s.queryNotifications(query, key)
}

func (s *PostgresStorage) QueueDepth() (int, error) {
	var depth int
	err := s.db.QueryRow(
		`SELECT COUNT(*)
		 FROM notifications
		 WHERE status IN ($1, $2)`,
		model.StatusPending,
		model.StatusQueued,
	).Scan(&depth)
	return depth, err
}

func (s *PostgresStorage) getNotificationsByIdempotencyKeyTx(tx *sql.Tx, key string) ([]*model.Notification, error) {
	query := `SELECT ` + notificationSelectColumns + ` FROM notifications WHERE idempotency_key = $1 ORDER BY created_at ASC`
	rows, err := tx.Query(query, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*model.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

func (s *PostgresStorage) CancelNotification(id string) (bool, error) {
	result, err := s.db.Exec(
		`UPDATE notifications SET status = $1, updated_at = $2 WHERE id = $3 AND status = $4`,
		model.StatusCancelled,
		time.Now().UTC(),
		id,
		model.StatusPending,
	)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

func (s *PostgresStorage) SaveTemplate(t *model.Template) error {
	_, err := s.db.Exec(
		`INSERT INTO templates (id, name, channel, content, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, channel = EXCLUDED.channel, content = EXCLUDED.content, updated_at = EXCLUDED.updated_at`,
		t.ID,
		t.Name,
		t.Channel,
		t.Content,
		t.CreatedAt.UTC(),
		t.UpdatedAt.UTC(),
	)
	return err
}

func (s *PostgresStorage) GetTemplateByID(id string) (*model.Template, error) {
	row := s.db.QueryRow(`SELECT id, name, channel, content, created_at, updated_at FROM templates WHERE id = $1`, id)
	return scanTemplate(row)
}

func (s *PostgresStorage) ListTemplates() ([]*model.Template, error) {
	rows, err := s.db.Query(`SELECT id, name, channel, content, created_at, updated_at FROM templates ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*model.Template{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	return result, rows.Err()
}

func (s *PostgresStorage) queryNotifications(query string, args ...any) ([]*model.Notification, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*model.Notification{}
	for rows.Next() {
		n, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

func scanNotification(row interface{ Scan(dest ...any) error }) (*model.Notification, error) {
	n := &model.Notification{}
	var errorMessage, externalMessageID, idempotencyKey, templateID, templateData sql.NullString
	var scheduledAt sql.NullTime
	if err := row.Scan(&n.ID, &n.BatchID, &n.Recipient, &n.Channel, &n.Content, &n.Priority, &n.Status, &errorMessage, &n.RetryCount, &externalMessageID, &idempotencyKey, &templateID, &templateData, &scheduledAt, &n.CreatedAt, &n.UpdatedAt); err != nil {
		return nil, err
	}
	if errorMessage.Valid {
		n.Error = errorMessage.String
	}
	if externalMessageID.Valid {
		n.ExternalMessageID = externalMessageID.String
	}
	if idempotencyKey.Valid {
		n.IdempotencyKey = idempotencyKey.String
	}
	if templateID.Valid {
		n.TemplateID = templateID.String
	}
	if templateData.Valid && templateData.String != "" {
		if err := json.Unmarshal([]byte(templateData.String), &n.TemplateData); err != nil {
			return nil, err
		}
	}
	if scheduledAt.Valid {
		value := scheduledAt.Time.UTC()
		n.ScheduledAt = &value
	}
	n.CreatedAt = n.CreatedAt.UTC()
	n.UpdatedAt = n.UpdatedAt.UTC()
	return n, nil
}

func scanTemplate(row interface{ Scan(dest ...any) error }) (*model.Template, error) {
	t := &model.Template{}
	if err := row.Scan(&t.ID, &t.Name, &t.Channel, &t.Content, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	return t, nil
}

func marshalTemplateData(data map[string]string) (any, error) {
	if len(data) == 0 {
		return nil, nil
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return string(payload), nil
}

func nullableTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func joinClauses(clauses []string, sep string) string {
	result := ""
	for i, clause := range clauses {
		if i > 0 {
			result += sep
		}
		result += clause
	}
	return result
}

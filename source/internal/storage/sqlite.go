package storage

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/yourusername/event-driven-notification-app/internal/model"
	_ "modernc.org/sqlite"
)

type SQLiteStorage struct {
	db *sql.DB
}

func NewSQLiteStorage(path string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	return &SQLiteStorage{db: db}, nil
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func (s *SQLiteStorage) Migrate() error {
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
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_batch_id ON notifications(batch_id);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);`,
		`CREATE INDEX IF NOT EXISTS idx_notifications_channel ON notifications(channel);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_notifications_idempotency_key ON notifications(idempotency_key);`,
	}

	for _, query := range queries {
		if _, err := s.db.Exec(query); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}

	return nil
}

func (s *SQLiteStorage) SaveNotification(n *model.Notification) error {
	stmt, err := s.db.Prepare(`INSERT INTO notifications (id, batch_id, recipient, channel, content, priority, status, error, retry_count, external_message_id, idempotency_key, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	var idempotencyKey interface{}
	if n.IdempotencyKey != "" {
		idempotencyKey = n.IdempotencyKey
	}

	_, err = stmt.Exec(
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
		idempotencyKey,
		n.CreatedAt,
		n.UpdatedAt,
	)
	return err
}

func (s *SQLiteStorage) GetNotificationByID(id string) (*model.Notification, error) {
	row := s.db.QueryRow(`SELECT id, batch_id, recipient, channel, content, priority, status, error, retry_count, external_message_id, created_at, updated_at FROM notifications WHERE id = ?`, id)
	return scanNotification(row)
}

func scanNotification(row interface{ Scan(dest ...any) error }) (*model.Notification, error) {
	n := &model.Notification{}
	var createdAt, updatedAt string
	if err := row.Scan(&n.ID, &n.BatchID, &n.Recipient, &n.Channel, &n.Content, &n.Priority, &n.Status, &n.Error, &n.RetryCount, &n.ExternalMessageID, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	var err error
	n.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, err
	}
	n.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
	if err != nil {
		return nil, err
	}
	return n, nil
}

func (s *SQLiteStorage) UpdateNotification(n *model.Notification) error {
	stmt, err := s.db.Prepare(`UPDATE notifications SET status = ?, error = ?, retry_count = ?, external_message_id = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(n.Status, n.Error, n.RetryCount, n.ExternalMessageID, n.UpdatedAt, n.ID)
	return err
}

func (s *SQLiteStorage) ListNotifications(filters map[string]string, page, size int) ([]*model.Notification, error) {
	query := `SELECT id, batch_id, recipient, channel, content, priority, status, error, retry_count, external_message_id, created_at, updated_at FROM notifications`
	clauses := []string{}
	args := []any{}

	if v, ok := filters["batch_id"]; ok && v != "" {
		clauses = append(clauses, `batch_id = ?`)
		args = append(args, v)
	}
	if v, ok := filters["status"]; ok && v != "" {
		clauses = append(clauses, `status = ?`)
		args = append(args, v)
	}
	if v, ok := filters["channel"]; ok && v != "" {
		clauses = append(clauses, `channel = ?`)
		args = append(args, v)
	}
	if v, ok := filters["recipient"]; ok && v != "" {
		clauses = append(clauses, `recipient = ?`)
		args = append(args, v)
	}

	if len(clauses) > 0 {
		query += " WHERE " + joinClauses(clauses, " AND ")
	}
	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, size, (page-1)*size)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*model.Notification{}
	for rows.Next() {
		n := &model.Notification{}
		var createdAt, updatedAt string
		if err := rows.Scan(&n.ID, &n.BatchID, &n.Recipient, &n.Channel, &n.Content, &n.Priority, &n.Status, &n.Error, &n.RetryCount, &n.ExternalMessageID, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		n.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
		if err != nil {
			return nil, err
		}
		n.UpdatedAt, err = time.Parse(time.RFC3339Nano, updatedAt)
		if err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, nil
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

func (s *SQLiteStorage) SetNotificationQueued(id string) error {
	stmt, err := s.db.Prepare(`UPDATE notifications SET status = ?, updated_at = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(model.StatusQueued, time.Now().UTC().Format(time.RFC3339Nano), id)
	return err
}

func (s *SQLiteStorage) GetPendingNotifications() ([]*model.Notification, error) {
	query := `SELECT id, batch_id, recipient, channel, content, priority, status, error, retry_count, external_message_id, created_at, updated_at FROM notifications WHERE status = ? ORDER BY priority ASC, created_at ASC`
	return s.queryNotifications(query, model.StatusPending)
}

func (s *SQLiteStorage) GetPendingNotificationsByBatch(batchID string) ([]*model.Notification, error) {
	query := `SELECT id, batch_id, recipient, channel, content, priority, status, error, retry_count, external_message_id, created_at, updated_at FROM notifications WHERE batch_id = ? ORDER BY priority ASC, created_at ASC`
	return s.queryNotifications(query, batchID)
}

func (s *SQLiteStorage) GetNotificationsByIdempotencyKey(key string) ([]*model.Notification, error) {
	query := `SELECT id, batch_id, recipient, channel, content, priority, status, error, retry_count, external_message_id, created_at, updated_at FROM notifications WHERE idempotency_key = ?`
	return s.queryNotifications(query, key)
}

func (s *SQLiteStorage) CancelNotification(id string) error {
	stmt, err := s.db.Prepare(`UPDATE notifications SET status = ?, updated_at = ? WHERE id = ? AND status IN (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(model.StatusCancelled, time.Now().UTC().Format(time.RFC3339Nano), id, model.StatusPending, model.StatusQueued, model.StatusFailed)
	return err
}

func (s *SQLiteStorage) queryNotifications(query string, args ...any) ([]*model.Notification, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := []*model.Notification{}
	for rows.Next() {
		n := &model.Notification{}
		var createdAt, updatedAt string
		if err := rows.Scan(&n.ID, &n.BatchID, &n.Recipient, &n.Channel, &n.Content, &n.Priority, &n.Status, &n.Error, &n.RetryCount, &n.ExternalMessageID, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		var parseErr error
		n.CreatedAt, parseErr = time.Parse(time.RFC3339Nano, createdAt)
		if parseErr != nil {
			return nil, parseErr
		}
		n.UpdatedAt, parseErr = time.Parse(time.RFC3339Nano, updatedAt)
		if parseErr != nil {
			return nil, parseErr
		}
		result = append(result, n)
	}
	return result, nil
}

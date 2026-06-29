package storage

import "github.com/yourusername/event-driven-notification-app/internal/model"

type Storage interface {
	Close() error
	Ping() error
	Migrate() error
	SaveNotification(n *model.Notification) error
	SaveNotificationsBatch(idempotencyKey string, notifications []*model.Notification) (bool, []*model.Notification, error)
	GetNotificationByID(id string) (*model.Notification, error)
	ClaimNotification(id string) (*model.Notification, bool, error)
	ClaimNextDueNotification(channel string) (*model.Notification, bool, error)
	UpdateNotification(n *model.Notification) error
	ListNotifications(filters map[string]string, page, size int) ([]*model.Notification, error)
	GetPendingNotifications() ([]*model.Notification, error)
	GetPendingNotificationsByBatch(batchID string) ([]*model.Notification, error)
	GetNotificationsByIdempotencyKey(key string) ([]*model.Notification, error)
	QueueDepth() (int, error)
	CancelNotification(id string) (bool, error)
	SaveTemplate(t *model.Template) error
	GetTemplateByID(id string) (*model.Template, error)
	ListTemplates() ([]*model.Template, error)
}

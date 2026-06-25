package storage

import "github.com/yourusername/event-driven-notification-app/internal/model"

type Storage interface {
	Migrate() error
	SaveNotification(n *model.Notification) error
	GetNotificationByID(id string) (*model.Notification, error)
	UpdateNotification(n *model.Notification) error
	ListNotifications(filters map[string]string, page, size int) ([]*model.Notification, error)
	GetNotificationsByIdempotencyKey(key string) ([]*model.Notification, error)
	CancelNotification(id string) error
}

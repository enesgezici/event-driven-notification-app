package queue

import (
	"container/heap"
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/yourusername/event-driven-notification-app/internal/metrics"
	"github.com/yourusername/event-driven-notification-app/internal/model"
	"github.com/yourusername/event-driven-notification-app/internal/storage"
)

func TestPopNextLockedSelectsHighestPriorityForChannel(t *testing.T) {
	manager := &Manager{
		queue:   make(priorityQueue, 0),
		metrics: metrics.NewCollector(),
	}
	base := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)

	heap.Push(&manager.queue, testQueueItem("sms-low", "sms", model.PriorityLow, base))
	heap.Push(&manager.queue, testQueueItem("email-high", "email", model.PriorityHigh, base.Add(time.Second)))
	heap.Push(&manager.queue, testQueueItem("sms-high", "sms", model.PriorityHigh, base.Add(2*time.Second)))

	item := manager.popNextLocked("sms")
	if item == nil {
		t.Fatal("expected an sms item")
	}
	if item.notification.ID != "sms-high" {
		t.Fatalf("expected sms-high, got %s", item.notification.ID)
	}
}

func TestPopNextLockedUsesEnqueueOrderWithinSamePriority(t *testing.T) {
	manager := &Manager{queue: make(priorityQueue, 0)}
	base := time.Date(2026, 6, 29, 9, 0, 0, 0, time.UTC)

	heap.Push(&manager.queue, testQueueItem("newer", "sms", model.PriorityNormal, base.Add(time.Second)))
	heap.Push(&manager.queue, testQueueItem("older", "sms", model.PriorityNormal, base))

	item := manager.popNextLocked("sms")
	if item == nil {
		t.Fatal("expected an sms item")
	}
	if item.notification.ID != "older" {
		t.Fatalf("expected older, got %s", item.notification.ID)
	}
}

func TestStopWorkersCancelsDelayedScheduledEnqueue(t *testing.T) {
	db := &queueTestStorage{}
	manager := NewManager(db, nil, metrics.NewCollector(), queueTestLogger())
	ctx, cancel := context.WithCancel(context.Background())
	manager.StartWorkers(ctx)

	scheduledAt := time.Now().UTC().Add(25 * time.Millisecond)
	manager.Enqueue(&model.Notification{
		ID:          "scheduled",
		BatchID:     "batch",
		Channel:     "sms",
		Priority:    model.PriorityNormal,
		Status:      model.StatusPending,
		ScheduledAt: &scheduledAt,
	})

	cancel()
	manager.StopWorkers()
	time.Sleep(50 * time.Millisecond)

	if depth := manager.QueueDepth(); depth != 0 {
		t.Fatalf("expected cancelled scheduled enqueue to leave queue empty, got depth %d", depth)
	}
}

func testQueueItem(id, channel string, priority model.NotificationPriority, enqueuedAt time.Time) *queueItem {
	return &queueItem{
		notification: &model.Notification{
			ID:       id,
			Channel:  channel,
			Priority: priority,
		},
		priority:   int(priority),
		enqueuedAt: enqueuedAt,
	}
}

func queueTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

var _ storage.Storage = (*queueTestStorage)(nil)

type queueTestStorage struct{}

func (s *queueTestStorage) Close() error                                 { return nil }
func (s *queueTestStorage) Migrate() error                               { return nil }
func (s *queueTestStorage) SaveNotification(n *model.Notification) error { return nil }
func (s *queueTestStorage) SaveNotificationsBatch(idempotencyKey string, notifications []*model.Notification) (bool, []*model.Notification, error) {
	return true, notifications, nil
}
func (s *queueTestStorage) GetNotificationByID(id string) (*model.Notification, error) {
	return nil, errQueueTestUnsupported
}
func (s *queueTestStorage) UpdateNotification(n *model.Notification) error { return nil }
func (s *queueTestStorage) ListNotifications(filters map[string]string, page, size int) ([]*model.Notification, error) {
	return nil, nil
}
func (s *queueTestStorage) GetPendingNotifications() ([]*model.Notification, error) { return nil, nil }
func (s *queueTestStorage) GetPendingNotificationsByBatch(batchID string) ([]*model.Notification, error) {
	return nil, nil
}
func (s *queueTestStorage) GetNotificationsByIdempotencyKey(key string) ([]*model.Notification, error) {
	return nil, nil
}
func (s *queueTestStorage) CancelNotification(id string) (bool, error) { return false, nil }
func (s *queueTestStorage) SetNotificationQueued(id string) error      { return nil }
func (s *queueTestStorage) SaveTemplate(tmpl *model.Template) error    { return nil }
func (s *queueTestStorage) GetTemplateByID(id string) (*model.Template, error) {
	return nil, errQueueTestUnsupported
}
func (s *queueTestStorage) ListTemplates() ([]*model.Template, error) { return nil, nil }

type queueTestError string

func (e queueTestError) Error() string {
	return string(e)
}

const errQueueTestUnsupported = queueTestError("unsupported queue test storage operation")

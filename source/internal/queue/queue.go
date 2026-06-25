package queue

import (
	"container/heap"
	"context"
	"log"
	"sync"
	"time"

	"github.com/yourusername/event-driven-notification-app/internal/metrics"
	"github.com/yourusername/event-driven-notification-app/internal/model"
	"github.com/yourusername/event-driven-notification-app/internal/provider"
	"github.com/yourusername/event-driven-notification-app/internal/storage"
)

type queueItem struct {
	notification *model.Notification
	index        int
	priority     int
	enqueuedAt   time.Time
}

type priorityQueue []*queueItem

func (pq priorityQueue) Len() int { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].priority == pq[j].priority {
		return pq[i].enqueuedAt.Before(pq[j].enqueuedAt)
	}
	return pq[i].priority < pq[j].priority
}
func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}
func (pq *priorityQueue) Push(x any) {
	item := x.(*queueItem)
	item.index = len(*pq)
	*pq = append(*pq, item)
}
func (pq *priorityQueue) Pop() any {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}

type Manager struct {
	db              *storage.SQLiteStorage
	provider        *provider.WebhookProvider
	metrics         *metrics.Collector
	logger          *log.Logger
	queue           priorityQueue
	lock            sync.Mutex
	channelTickers  map[string]*time.Ticker
	stopWorkersOnce sync.Once
}

func NewManager(db *storage.SQLiteStorage, providerClient *provider.WebhookProvider, collector *metrics.Collector, logger *log.Logger) *Manager {
	return &Manager{
		db:       db,
		provider: providerClient,
		metrics:  collector,
		logger:   logger,
		queue:    make(priorityQueue, 0),
		channelTickers: map[string]*time.Ticker{
			"sms":   time.NewTicker(10 * time.Millisecond),
			"email": time.NewTicker(10 * time.Millisecond),
			"push":  time.NewTicker(10 * time.Millisecond),
		},
	}
}

func (m *Manager) Enqueue(notification *model.Notification) {
	m.lock.Lock()
	heap.Push(&m.queue, &queueItem{
		notification: notification,
		priority:     int(notification.Priority),
		enqueuedAt:   time.Now().UTC(),
	})
	m.metrics.SetQueueDepth(len(m.queue))
	m.lock.Unlock()
}

func (m *Manager) StartWorkers(ctx context.Context) {
	m.loadPendingNotifications()
	for _, ch := range []string{"sms", "email", "push"} {
		ticker := m.channelTickers[ch]
		go func(channel string, ticker *time.Ticker) {
			for {
				select {
				case <-ctx.Done():
					ticker.Stop()
					return
				case <-ticker.C:
					m.processNext(channel)
				}
			}
		}(ch, ticker)
	}
}

func (m *Manager) loadPendingNotifications() {
	pending, err := m.db.GetPendingNotifications()
	if err != nil {
		m.logger.Printf("failed to load pending notifications: %v", err)
		return
	}
	for _, n := range pending {
		m.Enqueue(n)
	}
	m.logger.Printf("loaded %d pending notifications into queue", len(pending))
}

func (m *Manager) processNext(channel string) {
	m.lock.Lock()
	if len(m.queue) == 0 {
		m.metrics.SetQueueDepth(0)
		m.lock.Unlock()
		return
	}

	var item *queueItem
	for i, current := range m.queue {
		if current.notification.Channel == channel {
			item = current
			heap.Remove(&m.queue, i)
			break
		}
	}
	m.metrics.SetQueueDepth(len(m.queue))
	m.lock.Unlock()

	if item == nil {
		return
	}

	notification := item.notification
	current, err := m.db.GetNotificationByID(notification.ID)
	if err != nil {
		m.logger.Printf("fetch notification failed: %v", err)
		return
	}
	if current.Status == model.StatusCancelled {
		m.logger.Printf("notification %s cancelled before sending", notification.ID)
		return
	}

	if err := m.db.SetNotificationQueued(notification.ID); err != nil {
		m.logger.Printf("mark queued error: %v", err)
	}

	result, err := m.provider.Send(notification)
	if err != nil {
		notification.Status = model.StatusFailed
		notification.Error = err.Error()
		notification.RetryCount = current.RetryCount + 1
		m.metrics.IncrementFailed()
	} else {
		notification.Status = model.StatusSent
		notification.ExternalMessageID = result.MessageID
		m.metrics.IncrementSuccess()
	}
	notification.UpdatedAt = time.Now().UTC()

	if err := m.db.UpdateNotification(notification); err != nil {
		m.logger.Printf("update notification failed: %v", err)
	}
}

func (m *Manager) QueueDepth() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.queue)
}

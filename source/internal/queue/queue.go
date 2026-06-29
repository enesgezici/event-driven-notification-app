package queue

import (
	"container/heap"
	"context"
	"log/slog"
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
	db              storage.Storage
	provider        *provider.WebhookProvider
	metrics         *metrics.Collector
	logger          *slog.Logger
	queue           priorityQueue
	lock            sync.Mutex
	channelTickers  map[string]*time.Ticker
	workerCtx       context.Context
	workerCancel    context.CancelFunc
	workerDone      sync.WaitGroup
	stopWorkersOnce sync.Once
}

const (
	maxDeliveryAttempts   = 3
	maxMessagesPerSecond  = 100
	channelDispatchPeriod = time.Second / maxMessagesPerSecond
)

func NewManager(db storage.Storage, providerClient *provider.WebhookProvider, collector *metrics.Collector, logger *slog.Logger) *Manager {
	return &Manager{
		db:       db,
		provider: providerClient,
		metrics:  collector,
		logger:   logger,
		queue:    make(priorityQueue, 0),
		channelTickers: map[string]*time.Ticker{
			"sms":   time.NewTicker(channelDispatchPeriod),
			"email": time.NewTicker(channelDispatchPeriod),
			"push":  time.NewTicker(channelDispatchPeriod),
		},
	}
}

func (m *Manager) Enqueue(notification *model.Notification) {
	if notification.ScheduledAt != nil {
		delay := time.Until(notification.ScheduledAt.UTC())
		if delay > 0 {
			m.logger.Info("scheduled notification delayed", "correlation_id", notification.BatchID, "notification_id", notification.ID, "batch_id", notification.BatchID, "scheduled_at", notification.ScheduledAt.UTC(), "delay", delay.String())
			m.runDelayed(delay, func() {
				copy := *notification
				copy.ScheduledAt = nil
				m.Enqueue(&copy)
			})
			return
		}
	}

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
	workerCtx, cancel := context.WithCancel(ctx)
	m.lock.Lock()
	m.workerCtx = workerCtx
	m.workerCancel = cancel
	m.lock.Unlock()

	m.loadPendingNotifications()
	for _, ch := range []string{"sms", "email", "push"} {
		ticker := m.channelTickers[ch]
		m.workerDone.Add(1)
		go func(channel string, ticker *time.Ticker) {
			defer m.workerDone.Done()
			for {
				select {
				case <-workerCtx.Done():
					ticker.Stop()
					return
				case <-ticker.C:
					m.processNext(channel)
				}
			}
		}(ch, ticker)
	}
}

func (m *Manager) StopWorkers() {
	m.stopWorkersOnce.Do(func() {
		m.lock.Lock()
		cancel := m.workerCancel
		m.lock.Unlock()
		if cancel != nil {
			cancel()
		}
		m.workerDone.Wait()
	})
}

func (m *Manager) loadPendingNotifications() {
	pending, err := m.db.GetPendingNotifications()
	if err != nil {
		m.logger.Error("load pending notifications failed", "error", err)
		return
	}
	for _, n := range pending {
		m.Enqueue(n)
	}
	m.logger.Info("pending notifications loaded", "notification_count", len(pending))
}

func (m *Manager) processNext(channel string) {
	notification, ok := m.nextNotification(channel)
	if !ok {
		return
	}

	m.deliver(notification)
}

func (m *Manager) nextNotification(channel string) (*model.Notification, bool) {
	m.lock.Lock()
	item := m.popNextLocked(channel)
	m.metrics.SetQueueDepth(len(m.queue))
	m.lock.Unlock()

	if item != nil {
		claimed, ok, err := m.db.ClaimNotification(item.notification.ID)
		if err != nil {
			m.logger.Error("claim notification failed", "correlation_id", item.notification.BatchID, "notification_id", item.notification.ID, "error", err)
			return nil, false
		}
		if ok {
			return claimed, true
		}
	}

	claimed, ok, err := m.db.ClaimNextDueNotification(channel)
	if err != nil {
		m.logger.Error("claim next notification failed", "channel", channel, "error", err)
		return nil, false
	}
	return claimed, ok
}

func (m *Manager) deliver(notification *model.Notification) {
	startedAt := time.Now()
	result, err := m.provider.Send(notification)
	if err != nil {
		notification.Error = err.Error()
		notification.RetryCount++
		if notification.RetryCount < maxDeliveryAttempts {
			notification.Status = model.StatusPending
			retryAt := time.Now().UTC().Add(retryDelay(notification.RetryCount))
			notification.ScheduledAt = &retryAt
			notification.UpdatedAt = time.Now().UTC()
			if updateErr := m.db.UpdateNotification(notification); updateErr != nil {
				m.logger.Error("update retry notification failed", "correlation_id", notification.BatchID, "notification_id", notification.ID, "batch_id", notification.BatchID, "error", updateErr)
				return
			}
			delay := retryDelay(notification.RetryCount)
			m.metrics.IncrementRetry()
			m.logger.Warn("notification delivery retry scheduled", "correlation_id", notification.BatchID, "notification_id", notification.ID, "batch_id", notification.BatchID, "channel", notification.Channel, "retry_count", notification.RetryCount, "retry_in", delay.String(), "error", err)
			m.runDelayed(delay, func() {
				m.Enqueue(notification)
			})
			return
		}
		notification.Status = model.StatusFailed
		notification.ScheduledAt = nil
		m.metrics.RecordLatency(time.Since(startedAt))
		m.metrics.IncrementFailed()
	} else {
		notification.Status = model.StatusSent
		notification.Error = ""
		notification.ExternalMessageID = result.MessageID
		notification.ScheduledAt = nil
		m.metrics.RecordLatency(time.Since(startedAt))
		m.metrics.IncrementSuccess()
	}
	notification.UpdatedAt = time.Now().UTC()

	if err := m.db.UpdateNotification(notification); err != nil {
		m.logger.Error("update notification failed", "correlation_id", notification.BatchID, "notification_id", notification.ID, "batch_id", notification.BatchID, "error", err)
	}
}

func (m *Manager) popNextLocked(channel string) *queueItem {
	bestIndex := -1
	for i, current := range m.queue {
		if current.notification.Channel != channel {
			continue
		}
		if bestIndex == -1 || queueItemBefore(current, m.queue[bestIndex]) {
			bestIndex = i
		}
	}
	if bestIndex == -1 {
		return nil
	}
	return heap.Remove(&m.queue, bestIndex).(*queueItem)
}

func queueItemBefore(a, b *queueItem) bool {
	if a.priority == b.priority {
		return a.enqueuedAt.Before(b.enqueuedAt)
	}
	return a.priority < b.priority
}

func (m *Manager) runDelayed(delay time.Duration, fn func()) {
	m.lock.Lock()
	ctx := m.workerCtx
	m.lock.Unlock()
	if ctx == nil {
		ctx = context.Background()
	}

	timer := time.NewTimer(delay)
	go func() {
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
			fn()
		}
	}()
}

func retryDelay(retryCount int) time.Duration {
	if retryCount < 1 {
		return time.Second
	}
	return time.Duration(1<<(retryCount-1)) * time.Second
}

func (m *Manager) QueueDepth() int {
	m.lock.Lock()
	defer m.lock.Unlock()
	return len(m.queue)
}

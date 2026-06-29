package metrics

import (
	"encoding/json"
	"sync"
	"time"
)

type Collector struct {
	lock               sync.Mutex
	QueueDepth         int     `json:"queue_depth"`
	SuccessCount       int     `json:"success_count"`
	FailureCount       int     `json:"failure_count"`
	RetryCount         int     `json:"retry_count"`
	AverageLatencyMs   float64 `json:"average_latency_ms"`
	SuccessRate        float64 `json:"success_rate"`
	FailureRate        float64 `json:"failure_rate"`
	TotalDeliveries    int     `json:"total_deliveries"`
	totalLatencyMillis int64
	LastUpdated        time.Time `json:"last_updated"`
}

func NewCollector() *Collector {
	return &Collector{LastUpdated: time.Now().UTC()}
}

func (c *Collector) SetQueueDepth(depth int) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.QueueDepth = depth
	c.LastUpdated = time.Now().UTC()
}

func (c *Collector) IncrementSuccess() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.SuccessCount++
	c.recalculateLocked()
	c.LastUpdated = time.Now().UTC()
}

func (c *Collector) IncrementFailed() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.FailureCount++
	c.recalculateLocked()
	c.LastUpdated = time.Now().UTC()
}

func (c *Collector) IncrementRetry() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.RetryCount++
	c.LastUpdated = time.Now().UTC()
}

func (c *Collector) RecordLatency(duration time.Duration) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.totalLatencyMillis += duration.Milliseconds()
	c.recalculateLocked()
	c.LastUpdated = time.Now().UTC()
}

func (c *Collector) recalculateLocked() {
	c.TotalDeliveries = c.SuccessCount + c.FailureCount
	if c.TotalDeliveries == 0 {
		c.SuccessRate = 0
		c.FailureRate = 0
		c.AverageLatencyMs = 0
		return
	}
	c.SuccessRate = float64(c.SuccessCount) / float64(c.TotalDeliveries)
	c.FailureRate = float64(c.FailureCount) / float64(c.TotalDeliveries)
	c.AverageLatencyMs = float64(c.totalLatencyMillis) / float64(c.TotalDeliveries)
}

func (c *Collector) Snapshot() []byte {
	c.lock.Lock()
	defer c.lock.Unlock()
	payload, _ := json.Marshal(c)
	return payload
}

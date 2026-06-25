package metrics

import (
	"encoding/json"
	"sync"
	"time"
)

type Collector struct {
	lock         sync.Mutex
	QueueDepth   int       `json:"queue_depth"`
	SuccessCount int       `json:"success_count"`
	FailureCount int       `json:"failure_count"`
	LastUpdated  time.Time `json:"last_updated"`
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
	c.LastUpdated = time.Now().UTC()
}

func (c *Collector) IncrementFailed() {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.FailureCount++
	c.LastUpdated = time.Now().UTC()
}

func (c *Collector) Snapshot() []byte {
	c.lock.Lock()
	defer c.lock.Unlock()
	payload, _ := json.Marshal(c)
	return payload
}

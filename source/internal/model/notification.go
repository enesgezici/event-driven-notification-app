package model

import "time"

type NotificationStatus string

type NotificationPriority int

const (
	StatusPending   NotificationStatus = "pending"
	StatusQueued    NotificationStatus = "queued"
	StatusSent      NotificationStatus = "sent"
	StatusFailed    NotificationStatus = "failed"
	StatusCancelled NotificationStatus = "cancelled"
)

const (
	PriorityHigh   NotificationPriority = 1
	PriorityNormal NotificationPriority = 2
	PriorityLow    NotificationPriority = 3
)

type Notification struct {
	ID                string               `json:"id"`
	BatchID           string               `json:"batch_id,omitempty"`
	Recipient         string               `json:"recipient"`
	Channel           string               `json:"channel"`
	Content           string               `json:"content"`
	Priority          NotificationPriority `json:"priority"`
	Status            NotificationStatus   `json:"status"`
	Error             string               `json:"error,omitempty"`
	RetryCount        int                  `json:"retry_count"`
	ExternalMessageID string               `json:"external_message_id,omitempty"`
	IdempotencyKey    string               `json:"idempotency_key,omitempty"`
	TemplateID        string               `json:"template_id,omitempty"`
	TemplateData      map[string]string    `json:"template_data,omitempty"`
	ScheduledAt       *time.Time           `json:"scheduled_at,omitempty"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`
}

type Template struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Channel   string    `json:"channel"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func ParsePriority(value string) NotificationPriority {
	switch value {
	case "high":
		return PriorityHigh
	case "low":
		return PriorityLow
	default:
		return PriorityNormal
	}
}

func PriorityString(p NotificationPriority) string {
	switch p {
	case PriorityHigh:
		return "high"
	case PriorityLow:
		return "low"
	default:
		return "normal"
	}
}

func ValidateChannel(channel string) bool {
	switch channel {
	case "sms", "email", "push":
		return true
	default:
		return false
	}
}

func ValidateContent(channel, content string) bool {
	if content == "" {
		return false
	}
	if channel == "sms" && len(content) > 160 {
		return false
	}
	return true
}

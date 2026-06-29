package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/yourusername/event-driven-notification-app/internal/model"
)

type WebhookProvider struct {
	url    string
	client *http.Client
	logger *slog.Logger
}

type providerRequest struct {
	To      string `json:"to"`
	Channel string `json:"channel"`
	Content string `json:"content"`
}

type providerResponse struct {
	MessageID string `json:"messageId"`
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
}

type SendResult struct {
	MessageID string
}

func NewWebhookProvider(url string, logger *slog.Logger) *WebhookProvider {
	return &WebhookProvider{
		url: url,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,
	}
}

func (p *WebhookProvider) Send(notification *model.Notification) (*SendResult, error) {
	payload := providerRequest{
		To:      notification.Recipient,
		Channel: notification.Channel,
		Content: notification.Content,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, p.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Idempotency-Key", notification.ID)
	if notification.BatchID != "" {
		req.Header.Set("X-Correlation-ID", notification.BatchID)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected provider status %d", resp.StatusCode)
	}

	var result providerResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		// If response is not JSON (e.g., webhook.site default response), generate a messageId
		result.MessageID = uuid.NewString()
	}

	if result.MessageID == "" {
		result.MessageID = uuid.NewString()
	}
	return &SendResult{MessageID: result.MessageID}, nil
}

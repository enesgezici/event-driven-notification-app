package provider

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/yourusername/event-driven-notification-app/internal/model"
)

func TestWebhookProviderSendsExpectedPayloadAndHeaders(t *testing.T) {
	var gotPayload map[string]string
	var gotIdempotencyKey string
	var gotCorrelationID string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdempotencyKey = r.Header.Get("Idempotency-Key")
		gotCorrelationID = r.Header.Get("X-Correlation-ID")
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode provider request: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"messageId":"provider-message","status":"accepted","timestamp":"2026-06-29T09:00:00Z"}`))
	}))
	defer server.Close()

	client := NewWebhookProvider(server.URL, slog.New(slog.NewTextHandler(io.Discard, nil)))
	result, err := client.Send(&model.Notification{
		ID:        "notification-id",
		BatchID:   "batch-id",
		Recipient: "+905551234567",
		Channel:   "sms",
		Content:   "hello",
	})
	if err != nil {
		t.Fatalf("send notification: %v", err)
	}

	if result.MessageID != "provider-message" {
		t.Fatalf("expected provider-message, got %s", result.MessageID)
	}
	if gotPayload["to"] != "+905551234567" || gotPayload["channel"] != "sms" || gotPayload["content"] != "hello" {
		t.Fatalf("unexpected payload: %#v", gotPayload)
	}
	if gotIdempotencyKey != "notification-id" {
		t.Fatalf("expected idempotency header, got %q", gotIdempotencyKey)
	}
	if gotCorrelationID != "batch-id" {
		t.Fatalf("expected correlation header, got %q", gotCorrelationID)
	}
}

package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/yourusername/event-driven-notification-app/internal/metrics"
	"github.com/yourusername/event-driven-notification-app/internal/model"
	"github.com/yourusername/event-driven-notification-app/internal/queue"
	"github.com/yourusername/event-driven-notification-app/internal/storage"
)

type apiHandler struct {
	db      storage.Storage
	queue   *queue.Manager
	metrics *metrics.Collector
	logger  *slog.Logger
}

type notificationRequest struct {
	Recipient string `json:"recipient"`
	Channel   string `json:"channel"`
	Content   string `json:"content"`
	Priority  string `json:"priority,omitempty"`
}

type batchCreateRequest struct {
	Notifications []notificationRequest `json:"notifications"`
}

func NewRouter(db storage.Storage, queueManager *queue.Manager, metricsCollector *metrics.Collector, logger *slog.Logger) http.Handler {
	r := chi.NewRouter()
	h := &apiHandler{db: db, queue: queueManager, metrics: metricsCollector, logger: logger}

	r.Post("/notifications", h.createNotifications)
	r.Get("/notifications/{id}", h.getNotification)
	r.Get("/notifications", h.listNotifications)
	r.Delete("/notifications/{id}", h.cancelNotification)
	r.Get("/batches/{batch_id}/notifications", h.getBatchNotifications)
	r.Get("/health", h.health)
	r.Get("/metrics", h.getMetrics)

	return r
}

func (h *apiHandler) createNotifications(w http.ResponseWriter, r *http.Request) {
	var payload batchCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	if len(payload.Notifications) == 0 || len(payload.Notifications) > 1000 {
		h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "notifications array must contain between 1 and 1000 items"})
		return
	}

	batchID := uuid.NewString()
	correlationID := requestCorrelationID(r)
	idempotencyKey := r.Header.Get("Idempotency-Key")

	notifications := make([]*model.Notification, 0, len(payload.Notifications))
	for _, item := range payload.Notifications {
		channel := strings.ToLower(item.Channel)
		if item.Recipient == "" || item.Channel == "" || item.Content == "" {
			h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "recipient, channel and content are required"})
			return
		}
		if !model.ValidateChannel(channel) {
			h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid channel"})
			return
		}
		if !model.ValidateContent(channel, item.Content) {
			h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "content validation failed"})
			return
		}

		notif := &model.Notification{
			ID:         uuid.NewString(),
			BatchID:    batchID,
			Recipient:  item.Recipient,
			Channel:    channel,
			Content:    item.Content,
			Priority:   model.ParsePriority(strings.ToLower(item.Priority)),
			Status:     model.StatusPending,
			RetryCount: 0,
			CreatedAt:  time.Now().UTC(),
			UpdatedAt:  time.Now().UTC(),
		}
		if idempotencyKey != "" {
			notif.IdempotencyKey = idempotencyKey
		}
		notifications = append(notifications, notif)
	}

	created, saved, err := h.db.SaveNotificationsBatch(idempotencyKey, notifications)
	if err != nil {
		h.logger.Error("save notifications failed", "correlation_id", correlationID, "batch_id", batchID, "error", err)
		h.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save notification"})
		return
	}
	if !created {
		h.logger.Info("idempotent notification batch reused", "correlation_id", correlationID, "idempotency_key", idempotencyKey, "batch_id", savedBatchID(saved), "notification_count", len(saved))
		h.respondJSON(w, http.StatusOK, map[string]any{"batch_id": savedBatchID(saved), "notifications": saved})
		return
	}

	for _, notif := range saved {
		h.queue.Enqueue(notif)
	}

	h.logger.Info("notifications created", "correlation_id", correlationID, "batch_id", batchID, "notification_count", len(saved))
	h.respondJSON(w, http.StatusCreated, map[string]any{"batch_id": batchID, "notifications": saved})
}

func (h *apiHandler) getNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	notif, err := h.db.GetNotificationByID(id)
	if err != nil {
		h.respondJSON(w, http.StatusNotFound, map[string]string{"error": "notification not found"})
		return
	}
	h.respondJSON(w, http.StatusOK, notif)
}

func (h *apiHandler) listNotifications(w http.ResponseWriter, r *http.Request) {
	filters := map[string]string{
		"status":    r.URL.Query().Get("status"),
		"channel":   r.URL.Query().Get("channel"),
		"batch_id":  r.URL.Query().Get("batch_id"),
		"recipient": r.URL.Query().Get("recipient"),
	}
	from, err := parseTimeFilter(r.URL.Query().Get("from"), r.URL.Query().Get("start_date"))
	if err != nil {
		h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "from/start_date must be RFC3339"})
		return
	}
	to, err := parseTimeFilter(r.URL.Query().Get("to"), r.URL.Query().Get("end_date"))
	if err != nil {
		h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "to/end_date must be RFC3339"})
		return
	}
	filters["from"] = from
	filters["to"] = to
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 25
	}
	list, err := h.db.ListNotifications(filters, page, size)
	if err != nil {
		h.logger.Error("list notifications failed", "correlation_id", requestCorrelationID(r), "error", err)
		h.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list notifications"})
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"page": page, "size": size, "notifications": list})
}

func (h *apiHandler) cancelNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.CancelNotification(id); err != nil {
		h.logger.Error("cancel notification failed", "correlation_id", requestCorrelationID(r), "notification_id", id, "error", err)
		h.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not cancel notification"})
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"result": "cancelled"})
}

func (h *apiHandler) getBatchNotifications(w http.ResponseWriter, r *http.Request) {
	batchID := chi.URLParam(r, "batch_id")
	list, err := h.db.GetPendingNotificationsByBatch(batchID)
	if err != nil {
		h.logger.Error("get batch notifications failed", "correlation_id", requestCorrelationID(r), "batch_id", batchID, "error", err)
		h.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not get batch notifications"})
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"batch_id": batchID, "notifications": list})
}

func (h *apiHandler) health(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *apiHandler) getMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(h.metrics.Snapshot())
}

func (h *apiHandler) respondJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func parseTimeFilter(primary, alias string) (string, error) {
	value := primary
	if value == "" {
		value = alias
	}
	if value == "" {
		return "", nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", err
	}
	return parsed.UTC().Format(time.RFC3339Nano), nil
}

func requestCorrelationID(r *http.Request) string {
	if value := r.Header.Get("X-Correlation-ID"); value != "" {
		return value
	}
	if value := r.Header.Get("Idempotency-Key"); value != "" {
		return value
	}
	return uuid.NewString()
}

func savedBatchID(notifications []*model.Notification) string {
	if len(notifications) == 0 {
		return ""
	}
	return notifications[0].BatchID
}

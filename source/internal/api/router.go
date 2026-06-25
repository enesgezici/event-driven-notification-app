package api

import (
	"encoding/json"
	"log"
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
	db      *storage.SQLiteStorage
	queue   *queue.Manager
	metrics *metrics.Collector
	logger  *log.Logger
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

func NewRouter(db *storage.SQLiteStorage, queueManager *queue.Manager, metricsCollector *metrics.Collector, logger *log.Logger) http.Handler {
	r := chi.NewRouter()
	h := &apiHandler{db: db, queue: queueManager, metrics: metricsCollector, logger: logger}

	r.Post("/notifications", h.createNotifications)
	r.Get("/notifications/{id}", h.getNotification)
	r.Get("/notifications", h.listNotifications)
	r.Delete("/notifications/{id}", h.cancelNotification)
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
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey != "" {
		existing, err := h.db.GetNotificationsByIdempotencyKey(idempotencyKey)
		if err == nil && len(existing) > 0 {
			h.respondJSON(w, http.StatusOK, map[string]any{"batch_id": existing[0].BatchID, "notifications": existing})
			return
		}
	}

	notifications := make([]*model.Notification, 0, len(payload.Notifications))
	for _, item := range payload.Notifications {
		if item.Recipient == "" || item.Channel == "" || item.Content == "" {
			h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "recipient, channel and content are required"})
			return
		}
		if !model.ValidateChannel(item.Channel) {
			h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid channel"})
			return
		}
		if !model.ValidateContent(item.Channel, item.Content) {
			h.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "content validation failed"})
			return
		}

		notif := &model.Notification{
			ID:         uuid.NewString(),
			BatchID:    batchID,
			Recipient:  item.Recipient,
			Channel:    strings.ToLower(item.Channel),
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

	for _, notif := range notifications {
		if err := h.db.SaveNotification(notif); err != nil {
			h.logger.Printf("save notification error: %v", err)
			h.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save notification"})
			return
		}
		h.queue.Enqueue(notif)
	}

	h.respondJSON(w, http.StatusCreated, map[string]any{"batch_id": batchID, "notifications": notifications})
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
		h.logger.Printf("list notifications error: %v", err)
		h.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not list notifications"})
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]any{"page": page, "size": size, "notifications": list})
}

func (h *apiHandler) cancelNotification(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.db.CancelNotification(id); err != nil {
		h.logger.Printf("cancel notification error: %v", err)
		h.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not cancel notification"})
		return
	}
	h.respondJSON(w, http.StatusOK, map[string]string{"result": "cancelled"})
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

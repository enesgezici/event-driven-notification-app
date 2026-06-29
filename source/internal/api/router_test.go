package api

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/yourusername/event-driven-notification-app/internal/metrics"
	"github.com/yourusername/event-driven-notification-app/internal/model"
	"github.com/yourusername/event-driven-notification-app/internal/storage"
)

func TestParseTimeFilter(t *testing.T) {
	got, err := parseTimeFilter("", "2026-06-25T12:30:00Z")
	if err != nil {
		t.Fatalf("parse time filter: %v", err)
	}

	want := time.Date(2026, 6, 25, 12, 30, 0, 0, time.UTC).Format(time.RFC3339Nano)
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestParseTimeFilterRejectsInvalidDate(t *testing.T) {
	if _, err := parseTimeFilter("not-a-date", ""); err == nil {
		t.Fatal("expected invalid date to return an error")
	}
}

func TestParseOptionalFutureTime(t *testing.T) {
	future := time.Now().UTC().Add(time.Hour).Format(time.RFC3339)
	got, err := parseOptionalFutureTime(future)
	if err != nil {
		t.Fatalf("parse optional future time: %v", err)
	}
	if got == nil {
		t.Fatal("expected scheduled time")
	}
}

func TestParseOptionalFutureTimeRejectsPast(t *testing.T) {
	past := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	if _, err := parseOptionalFutureTime(past); err == nil {
		t.Fatal("expected past scheduled_at to return an error")
	}
}

func TestCompileTemplate(t *testing.T) {
	tmpl, err := compileTemplate("Hello {{.name}}")
	if err != nil {
		t.Fatalf("compile template: %v", err)
	}
	if tmpl == nil {
		t.Fatal("expected compiled template")
	}
}

func TestCancelNotificationReturnsConflictWhenNotCancelled(t *testing.T) {
	db := &routerTestStorage{cancelled: false}
	router := NewRouter(db, nil, metrics.NewCollector(), testLogger())

	req := httptest.NewRequest(http.MethodDelete, "/notifications/missing", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusConflict {
		t.Fatalf("expected status %d, got %d", http.StatusConflict, res.Code)
	}
}

func TestCancelNotificationReturnsOKWhenCancelled(t *testing.T) {
	db := &routerTestStorage{cancelled: true}
	router := NewRouter(db, nil, metrics.NewCollector(), testLogger())

	req := httptest.NewRequest(http.MethodDelete, "/notifications/pending", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, res.Code)
	}
}

func TestHealthReturnsUnavailableWhenStoragePingFails(t *testing.T) {
	db := &routerTestStorage{pingErr: errors.New("database down")}
	router := NewRouter(db, nil, metrics.NewCollector(), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	res := httptest.NewRecorder()

	router.ServeHTTP(res, req)

	if res.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status %d, got %d", http.StatusServiceUnavailable, res.Code)
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

var _ storage.Storage = (*routerTestStorage)(nil)

type routerTestStorage struct {
	cancelled bool
	pingErr   error
}

func (s *routerTestStorage) Close() error                                 { return nil }
func (s *routerTestStorage) Ping() error                                  { return s.pingErr }
func (s *routerTestStorage) Migrate() error                               { return nil }
func (s *routerTestStorage) SaveNotification(n *model.Notification) error { return nil }
func (s *routerTestStorage) SaveNotificationsBatch(idempotencyKey string, notifications []*model.Notification) (bool, []*model.Notification, error) {
	return true, notifications, nil
}
func (s *routerTestStorage) GetNotificationByID(id string) (*model.Notification, error) {
	return nil, errTestStorageUnsupported
}
func (s *routerTestStorage) ClaimNotification(id string) (*model.Notification, bool, error) {
	return nil, false, nil
}
func (s *routerTestStorage) ClaimNextDueNotification(channel string) (*model.Notification, bool, error) {
	return nil, false, nil
}
func (s *routerTestStorage) UpdateNotification(n *model.Notification) error { return nil }
func (s *routerTestStorage) ListNotifications(filters map[string]string, page, size int) ([]*model.Notification, error) {
	return nil, nil
}
func (s *routerTestStorage) GetPendingNotifications() ([]*model.Notification, error) { return nil, nil }
func (s *routerTestStorage) GetPendingNotificationsByBatch(batchID string) ([]*model.Notification, error) {
	return nil, nil
}
func (s *routerTestStorage) GetNotificationsByIdempotencyKey(key string) ([]*model.Notification, error) {
	return nil, nil
}
func (s *routerTestStorage) QueueDepth() (int, error)                   { return 0, nil }
func (s *routerTestStorage) CancelNotification(id string) (bool, error) { return s.cancelled, nil }
func (s *routerTestStorage) SaveTemplate(tmpl *model.Template) error    { return nil }
func (s *routerTestStorage) GetTemplateByID(id string) (*model.Template, error) {
	return nil, errTestStorageUnsupported
}
func (s *routerTestStorage) ListTemplates() ([]*model.Template, error) { return nil, nil }

type testStorageError string

func (e testStorageError) Error() string {
	return string(e)
}

const errTestStorageUnsupported = testStorageError("unsupported test storage operation")

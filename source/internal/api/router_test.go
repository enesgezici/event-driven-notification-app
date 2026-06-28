package api

import (
	"testing"
	"time"
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

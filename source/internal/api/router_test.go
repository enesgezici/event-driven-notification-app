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

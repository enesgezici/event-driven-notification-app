package model

import (
	"strings"
	"testing"
)

func TestValidateChannel(t *testing.T) {
	for _, channel := range []string{"sms", "email", "push"} {
		if !ValidateChannel(channel) {
			t.Fatalf("expected %s to be valid", channel)
		}
	}

	if ValidateChannel("fax") {
		t.Fatal("expected unsupported channel to be invalid")
	}
}

func TestValidateContent(t *testing.T) {
	if !ValidateContent("email", "hello") {
		t.Fatal("expected non-empty email content to be valid")
	}

	if ValidateContent("sms", strings.Repeat("a", 161)) {
		t.Fatal("expected SMS content over 160 chars to be invalid")
	}

	if ValidateContent("push", "") {
		t.Fatal("expected empty push content to be invalid")
	}
}

func TestParsePriority(t *testing.T) {
	if ParsePriority("high") != PriorityHigh {
		t.Fatal("expected high priority")
	}
	if ParsePriority("low") != PriorityLow {
		t.Fatal("expected low priority")
	}
	if ParsePriority("unknown") != PriorityNormal {
		t.Fatal("expected unknown priority to default to normal")
	}
}

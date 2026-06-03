package logger

import (
	"dnsway-pc/internal/models"
	"testing"
	"time"
)

func TestNewDecisionLogger(t *testing.T) {
	l := NewDecisionLogger()
	if l == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLogAndGetHistory(t *testing.T) {
	l := NewDecisionLogger()
	l.Log(&models.FilterDecision{
		Domain:    "example.com",
		ProfileID: "test",
		Decision:  models.Allow,
		Reason:    "P6: Default Allow",
		Timestamp: time.Now(),
	})

	history := l.GetHistory()
	if len(history) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(history))
	}
	if history[0]["domain"] != "example.com" {
		t.Errorf("expected example.com, got %v", history[0]["domain"])
	}
	if history[0]["decision"] != "ALLOW" {
		t.Errorf("expected ALLOW, got %v", history[0]["decision"])
	}
}

func TestLogMultipleEntries(t *testing.T) {
	l := NewDecisionLogger()
	for i := 0; i < 5; i++ {
		l.Log(&models.FilterDecision{
			Domain:    "test.com",
			ProfileID: "test",
			Decision:  models.Block,
			Reason:    "test",
			Timestamp: time.Now(),
		})
	}

	history := l.GetHistory()
	if len(history) != 5 {
		t.Errorf("expected 5 entries, got %d", len(history))
	}
}

func TestGetHistoryMax100(t *testing.T) {
	l := NewDecisionLogger()
	for i := 0; i < 120; i++ {
		l.Log(&models.FilterDecision{
			Domain:    "test.com",
			ProfileID: "test",
			Decision:  models.Allow,
			Reason:    "test",
			Timestamp: time.Now(),
		})
	}

	history := l.GetHistory()
	if len(history) > 100 {
		t.Errorf("history capped at 100, got %d", len(history))
	}
}

func TestGetTopBlocked(t *testing.T) {
	l := NewDecisionLogger()
	now := time.Now()

	entries := []*models.FilterDecision{
		{Domain: "bad.com", ProfileID: "test", Decision: models.Block, Reason: "P2: Denylist", Timestamp: now},
		{Domain: "bad.com", ProfileID: "test", Decision: models.Block, Reason: "P2: Denylist", Timestamp: now},
		{Domain: "bad.com", ProfileID: "test", Decision: models.Block, Reason: "P2: Denylist", Timestamp: now},
		{Domain: "evil.com", ProfileID: "test", Decision: models.Block, Reason: "P2: Denylist", Timestamp: now},
		{Domain: "good.com", ProfileID: "test", Decision: models.Allow, Reason: "P6: Allow", Timestamp: now},
	}
	for _, e := range entries {
		l.Log(e)
	}

	stats := l.GetTopBlocked()
	if stats["bad.com"] != 3 {
		t.Errorf("expected 3 blocks for bad.com, got %d", stats["bad.com"])
	}
	if stats["evil.com"] != 1 {
		t.Errorf("expected 1 block for evil.com, got %d", stats["evil.com"])
	}
	if _, exists := stats["good.com"]; exists {
		t.Error("good.com should not be in blocked stats")
	}
}

func TestGetTopBlockedEmptyHistory(t *testing.T) {
	l := NewDecisionLogger()
	stats := l.GetTopBlocked()
	if len(stats) != 0 {
		t.Errorf("expected empty stats, got %v", stats)
	}
}

func TestFormatDecision(t *testing.T) {
	l := NewDecisionLogger()

	tests := []struct {
		dt   models.DecisionType
		want string
	}{
		{models.Allow, "ALLOW"},
		{models.Block, "BLOCK"},
		{models.Redirect, "REDIRECT"},
		{models.DecisionType(99), "UNKNOWN"},
	}
	for _, tc := range tests {
		got := l.formatDecision(tc.dt)
		if got != tc.want {
			t.Errorf("formatDecision(%d) = %s, want %s", tc.dt, got, tc.want)
		}
	}
}

func TestLogConcurrencySafety(t *testing.T) {
	l := NewDecisionLogger()
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			l.Log(&models.FilterDecision{
				Domain:    "concurrent.com",
				ProfileID: "test",
				Decision:  models.Block,
				Timestamp: time.Now(),
			})
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	history := l.GetHistory()
	if len(history) != 10 {
		t.Errorf("expected 10 entries, got %d", len(history))
	}
}

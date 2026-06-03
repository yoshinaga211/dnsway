package logger

import (
	"dnsway-pc/internal/models"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// DecisionLogger 负责将决策结果持久化
type DecisionLogger struct {
	history []map[string]interface{}
	mu      sync.Mutex
}

func NewDecisionLogger() *DecisionLogger {
	return &DecisionLogger{
		history: make([]map[string]interface{}, 0),
	}
}

func (l *DecisionLogger) Log(d *models.FilterDecision) {
	entry := map[string]interface{}{
		"ts":         d.Timestamp.Format(time.RFC3339),
		"domain":     d.Domain,
		"profile_id": d.ProfileID,
		"decision":   l.formatDecision(d.Decision),
		"reason":     d.Reason,
		"categories": d.Categories,
		"latency_ms": float64(d.Latency.Microseconds()) / 1000.0,
	}

	data, _ := json.Marshal(entry)
	fmt.Printf("[DECISION_LOG] %s\n", string(data))

	l.mu.Lock()
	defer l.mu.Unlock()
	l.history = append(l.history, entry)
	if len(l.history) > 100 {
		l.history = l.history[1:]
	}
}

// GetHistory 获取内存中的历史记录
func (l *DecisionLogger) GetHistory() []map[string]interface{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.history
}

// GetTopBlocked 获取拦截统计排行榜
func (l *DecisionLogger) GetTopBlocked() map[string]int {
	l.mu.Lock()
	defer l.mu.Unlock()

	stats := make(map[string]int)
	for _, entry := range l.history {
		if entry["decision"] == "BLOCK" {
			domain := entry["domain"].(string)
			stats[domain]++
		}
	}
	return stats
}

func (l *DecisionLogger) formatDecision(dt models.DecisionType) string {
	switch dt {
	case models.Allow: return "ALLOW"
	case models.Block: return "BLOCK"
	case models.Redirect: return "REDIRECT"
	default: return "UNKNOWN"
	}
}

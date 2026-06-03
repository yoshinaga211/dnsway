package models

import (
	"time"
)

// DecisionType defines the outcome of a filter decision.
type DecisionType int

const (
	Allow    DecisionType = 0 // Allow the query
	Block    DecisionType = 1 // Block (return NXDOMAIN)
	REWRITE  DecisionType = 2 // Rewrite domain (SafeSearch CNAME)
	Redirect DecisionType = 2 // Alias for REWRITE
)

// FilterDecision is the result of a DNS filter evaluation.
type FilterDecision struct {
	Domain     string        `json:"domain"`
	ProfileID  string        `json:"profile_id"`
	Decision   DecisionType  `json:"decision"`
	Reason     string        `json:"reason"`
	Categories []string      `json:"categories,omitempty"`
	Latency    time.Duration `json:"latency"`
	Timestamp  time.Time     `json:"timestamp"`
	ClientIP   string        `json:"client_ip,omitempty"`
}

// TargetType defines the type of a filter target.
type TargetType string

const (
	TargetDomain   TargetType = "DOMAIN"
	TargetCategory TargetType = "CATEGORY"
)

// FilterTarget is an individual target for a time window.
type FilterTarget struct {
	Type  TargetType `json:"type"`
	Value string     `json:"value"`
}

// TimeWindow defines an entertainment time window.
type TimeWindow struct {
	ID         string         `json:"id"`
	ProfileID  string         `json:"profile_id"`
	Name       string         `json:"name"`
	DaysOfWeek []int          `json:"days_of_week"`
	StartTime  string         `json:"start_time"`
	EndTime    string         `json:"end_time"`
	Timezone   string         `json:"timezone"`
	Targets    []FilterTarget `json:"targets"`
}

// TimeWindowRequest is the API request body for creating a time window.
type TimeWindowRequest struct {
	Name       string `json:"name"`
	DaysOfWeek []int  `json:"days_of_week"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Timezone   string `json:"timezone"`
	TargetType string `json:"target_type"`
	TargetVal  string `json:"target_value"`
}

// DenylistEntry is a single denylist item.
type DenylistEntry struct {
	Domain string `json:"domain"`
	Reason string `json:"reason,omitempty"`
}

// SettingsUpdate is the request to update profile settings.
type SettingsUpdate struct {
	SafeSearch          *bool   `json:"safe_search,omitempty"`
	YoutubeRestricted   *bool   `json:"youtube_restricted,omitempty"`
	BypassPrevention    *bool   `json:"bypass_prevention,omitempty"`
	LogRetention        *int    `json:"log_retention_days,omitempty"`
	PhishingProtection      *bool   `json:"phishing_protection,omitempty"`
	AIThreatDetection       *bool   `json:"ai_threat_detection,omitempty"`
	CryptoJackingProtection *bool   `json:"crypto_jacking_protection,omitempty"`
	SuspectTLDBlocking      *bool   `json:"suspect_tld_blocking,omitempty"`
	StorageRegion           *string `json:"storage_region,omitempty"`
}

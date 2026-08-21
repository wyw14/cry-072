package domain

import (
	"maps"
	"time"
)

type AuditEvent struct {
	ID            string         `json:"id"`
	HazardID      string         `json:"hazard_id"`
	SiteID        string         `json:"site_id"`
	EventType     string         `json:"event_type"`
	ActorID       string         `json:"actor_id"`
	ActorRole     Role           `json:"actor_role"`
	RuleID        string         `json:"rule_id"`
	RuleVersion   int64          `json:"rule_version"`
	PreviousState HazardState    `json:"previous_state"`
	CurrentState  HazardState    `json:"current_state"`
	Reason        string         `json:"reason"`
	RequestID     string         `json:"request_id"`
	SensitiveKeys []string       `json:"-"`
	Details       map[string]any `json:"details"`
	OccurredAt    time.Time      `json:"occurred_at"`
}

func (e AuditEvent) PublicDetails() map[string]any {
	redacted := maps.Clone(e.Details)
	if redacted == nil {
		redacted = make(map[string]any)
	}
	for _, key := range e.SensitiveKeys {
		if _, exists := redacted[key]; exists {
			redacted[key] = "***"
		}
	}
	return redacted
}

type Page[T any] struct {
	Items      []T `json:"items"`
	Page       int `json:"page"`
	PageSize   int `json:"page_size"`
	Total      int `json:"total"`
	TotalPages int `json:"total_pages"`
}

func NewPage[T any](items []T, page, pageSize, total int) Page[T] {
	pages := 0
	if pageSize > 0 {
		pages = (total + pageSize - 1) / pageSize
	}
	return Page[T]{Items: items, Page: page, PageSize: pageSize, Total: total, TotalPages: pages}
}

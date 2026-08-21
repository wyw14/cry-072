package domain

import "time"

type QueueEntry struct {
	HazardID       string    `json:"hazard_id"`
	Queue          QueueKind `json:"queue"`
	OwnerID        string    `json:"owner_id"`
	DueAt          time.Time `json:"due_at"`
	EscalationTier int       `json:"escalation_tier"`
	LastNotifiedAt time.Time `json:"last_notified_at"`
	NextNotifyAt   time.Time `json:"next_notify_at"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	Version        int64     `json:"version"`
}

func (q QueueEntry) Overdue(now time.Time) bool {
	return !q.DueAt.IsZero() && now.After(q.DueAt)
}

func (q QueueEntry) NotificationDue(now time.Time) bool {
	return q.AcknowledgedAt.IsZero() && !q.NextNotifyAt.IsZero() && !now.Before(q.NextNotifyAt)
}

type QueueFilter struct {
	Queue       QueueKind   `json:"queue"`
	OwnerID     string      `json:"owner_id"`
	State       HazardState `json:"state"`
	RiskLevel   RiskLevel   `json:"risk_level"`
	SiteID      string      `json:"site_id"`
	OverdueOnly bool        `json:"overdue_only"`
	Page        int         `json:"page"`
	PageSize    int         `json:"page_size"`
	Sort        string      `json:"sort"`
	Descending  bool        `json:"descending"`
}

type queueViewPolicy struct {
	defaultPageSize int
	maximumPageSize int
	sortColumns     map[string]struct{}
}

var queueViews = queueViewPolicy{
	defaultPageSize: 20,
	maximumPageSize: 100,
	sortColumns: map[string]struct{}{
		"created_at": {}, "updated_at": {}, "due_at": {}, "risk_score": {},
	},
}

func (p queueViewPolicy) apply(filter QueueFilter) QueueFilter {
	filter.Page = max(1, filter.Page)
	requestedSize := filter.PageSize
	if requestedSize <= 0 {
		requestedSize = p.defaultPageSize
	}
	filter.PageSize = min(requestedSize, p.maximumPageSize)
	if _, supported := p.sortColumns[filter.Sort]; !supported {
		filter.Sort = "due_at"
	}
	return filter
}

func (f QueueFilter) Normalize() QueueFilter {
	return queueViews.apply(f)
}

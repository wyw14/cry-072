package domain

import "time"

type Notification struct {
	ID          string                  `json:"id"`
	HazardID    string                  `json:"hazard_id"`
	Destination NotificationDestination `json:"destination"`
	Notice      EscalationNotice        `json:"notice"`
	Delivery    DeliveryState           `json:"delivery"`
	CreatedAt   time.Time               `json:"created_at"`
}

type NotificationDestination struct {
	OperatorID string `json:"operator_id"`
	Adapter    string `json:"adapter"`
}

type EscalationNotice struct {
	Template string `json:"template"`
	Tier     int    `json:"tier"`
	Body     string `json:"body"`
}

type DeliveryState struct {
	DeduplicationKey string    `json:"deduplication_key"`
	SentAt           time.Time `json:"sent_at"`
	AcknowledgedAt   time.Time `json:"acknowledged_at"`
}

func (n *Notification) MarkSent(at time.Time) { n.Delivery.SentAt = at.UTC() }

type NotificationAttempt struct {
	NotificationID string    `json:"notification_id"`
	Adapter        string    `json:"adapter"`
	Succeeded      bool      `json:"succeeded"`
	ErrorCode      string    `json:"error_code"`
	AttemptedAt    time.Time `json:"attempted_at"`
}

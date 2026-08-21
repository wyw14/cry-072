package domain

import "time"

type QueueMetrics struct {
	Queue             QueueKind `json:"queue"`
	OpenCount         int       `json:"open_count"`
	OverdueCount      int       `json:"overdue_count"`
	AverageHandleMins float64   `json:"average_handle_minutes"`
	ResolvedOnTime    int       `json:"resolved_on_time"`
	ResolvedTotal     int       `json:"resolved_total"`
	OnTimeRate        float64   `json:"on_time_rate"`
}

type RiskDistribution struct {
	Level RiskLevel `json:"level"`
	Count int       `json:"count"`
}

type Anomaly struct {
	Code        string    `json:"code"`
	SiteID      string    `json:"site_id"`
	Description string    `json:"description"`
	Observed    float64   `json:"observed"`
	Threshold   float64   `json:"threshold"`
	DetectedAt  time.Time `json:"detected_at"`
}

type AnalyticsSnapshot struct {
	From             time.Time          `json:"from"`
	To               time.Time          `json:"to"`
	Normal           QueueMetrics       `json:"normal"`
	Focus            QueueMetrics       `json:"focus"`
	RiskDistribution []RiskDistribution `json:"risk_distribution"`
	Anomalies        []Anomaly          `json:"anomalies"`
	GeneratedAt      time.Time          `json:"generated_at"`
}

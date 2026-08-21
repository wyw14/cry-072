package domain

import (
	"strings"
	"time"
)

type RemediationTask struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	OwnerID     string    `json:"owner_id"`
	DueAt       time.Time `json:"due_at"`
	CompletedAt time.Time `json:"completed_at"`
	EvidenceIDs []string  `json:"evidence_ids"`
}

type RemediationPlan struct {
	ID             string            `json:"id"`
	HazardID       string            `json:"hazard_id"`
	Summary        string            `json:"summary"`
	TemporaryBlock bool              `json:"temporary_block"`
	Tasks          []RemediationTask `json:"tasks"`
	CreatedBy      string            `json:"created_by"`
	ApprovedBy     string            `json:"approved_by"`
	Version        int64             `json:"version"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func (p RemediationPlan) Validate() error {
	if strings.TrimSpace(p.HazardID) == "" || strings.TrimSpace(p.Summary) == "" {
		return NewValidationError("plan", "整改计划必须关联隐患并填写摘要")
	}
	if len(p.Tasks) == 0 {
		return NewValidationError("tasks", "整改计划至少包含一项任务")
	}
	for _, task := range p.Tasks {
		if strings.TrimSpace(task.Title) == "" || task.OwnerID == "" || task.DueAt.IsZero() {
			return NewValidationError("tasks", "整改任务必须填写标题、负责人和截止时间")
		}
	}
	return nil
}

func (p RemediationPlan) Complete() bool {
	if len(p.Tasks) == 0 {
		return false
	}
	for _, task := range p.Tasks {
		if task.CompletedAt.IsZero() {
			return false
		}
	}
	return true
}

type Reinspection struct {
	ID             string    `json:"id"`
	HazardID       string    `json:"hazard_id"`
	InspectorID    string    `json:"inspector_id"`
	Passed         bool      `json:"passed"`
	Notes          string    `json:"notes"`
	EvidenceIDs    []string  `json:"evidence_ids"`
	RestoreSite    bool      `json:"restore_site"`
	ObservedScore  int       `json:"observed_score"`
	IdempotencyKey string    `json:"idempotency_key"`
	CompletedAt    time.Time `json:"completed_at"`
}

func (r Reinspection) Validate() error {
	if r.HazardID == "" || r.InspectorID == "" {
		return NewValidationError("reinspection", "复检必须关联隐患和复检人")
	}
	if strings.TrimSpace(r.Notes) == "" {
		return NewValidationError("notes", "复检结论不能为空")
	}
	if r.ObservedScore < 0 || r.ObservedScore > 100 {
		return NewValidationError("observed_score", "复检风险分必须在 0 到 100 之间")
	}
	if r.Passed && len(r.EvidenceIDs) == 0 {
		return NewValidationError("evidence_ids", "通过复检必须提供证据")
	}
	return nil
}

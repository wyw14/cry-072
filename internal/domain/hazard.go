package domain

import (
	"strings"
	"time"
)

type Evidence struct {
	ID          string    `json:"id"`
	HazardID    string    `json:"hazard_id"`
	Kind        string    `json:"kind"`
	FileName    string    `json:"file_name"`
	ContentType string    `json:"content_type"`
	StorageKey  string    `json:"storage_key"`
	SizeBytes   int64     `json:"size_bytes"`
	Description string    `json:"description"`
	UploadedBy  string    `json:"uploaded_by"`
	CreatedAt   time.Time `json:"created_at"`
}

type Hazard struct {
	ID                   string      `json:"id"`
	InspectionID         string      `json:"inspection_id"`
	SiteID               string      `json:"site_id"`
	FacilityID           string      `json:"facility_id"`
	InspectionItemID     string      `json:"inspection_item_id"`
	RiskCategoryID       string      `json:"risk_category_id"`
	Title                string      `json:"title"`
	Description          string      `json:"description"`
	Severity             int         `json:"severity"`
	Likelihood           int         `json:"likelihood"`
	RiskScore            int         `json:"risk_score"`
	RiskLevel            RiskLevel   `json:"risk_level"`
	ImpactScopes         []string    `json:"impact_scopes"`
	InitialAction        string      `json:"initial_action"`
	IdempotencyKey       string      `json:"idempotency_key"`
	State                HazardState `json:"state"`
	Queue                QueueKind   `json:"queue"`
	Escalated            bool        `json:"escalated"`
	EscalationReason     string      `json:"escalation_reason"`
	MatchedRuleID        string      `json:"matched_rule_id"`
	MatchedRuleVersion   int64       `json:"matched_rule_version"`
	MatchedRuleEvidence  []string    `json:"matched_rule_evidence"`
	OwnerID              string      `json:"owner_id"`
	DueAt                time.Time   `json:"due_at"`
	LastReinspectionID   string      `json:"last_reinspection_id"`
	PassedReinspectionAt *time.Time  `json:"passed_reinspection_at,omitempty"`
	Version              int64       `json:"version"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`
}

func (h Hazard) ValidateNew() error {
	violations := make([]FieldViolation, 0, 4)
	if strings.TrimSpace(h.SiteID) == "" {
		violations = append(violations, FieldViolation{Field: "site_id", Message: "场地不能为空"})
	}
	if strings.TrimSpace(h.RiskCategoryID) == "" {
		violations = append(violations, FieldViolation{Field: "risk_category_id", Message: "风险分类不能为空"})
	}
	if strings.TrimSpace(h.Title) == "" {
		violations = append(violations, FieldViolation{Field: "title", Message: "隐患标题不能为空"})
	}
	if h.Severity < 1 || h.Severity > 5 || h.Likelihood < 1 || h.Likelihood > 5 {
		violations = append(violations, FieldViolation{Field: "risk", Message: "严重程度和可能性必须在 1 到 5 之间"})
	}
	if len(violations) > 0 {
		return ValidationError{Violations: violations}
	}
	return nil
}

func (h Hazard) IsFocus() bool { return h.Queue == QueueFocus || h.Escalated }

func (h Hazard) CanClose() error {
	if h.State != StateResolved {
		return ErrInvalidTransition
	}
	if h.IsFocus() && h.PassedReinspectionAt == nil {
		return NewValidationError("reinspection", "重点隐患必须通过复检后才能关闭")
	}
	return nil
}

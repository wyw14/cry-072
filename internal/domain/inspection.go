package domain

import (
	"fmt"
	"time"
)

type Inspection struct {
	ID             string             `json:"id"`
	SiteID         string             `json:"site_id"`
	FacilityID     string             `json:"facility_id"`
	InspectorID    string             `json:"inspector_id"`
	StartedAt      time.Time          `json:"started_at"`
	CompletedAt    time.Time          `json:"completed_at"`
	IdempotencyKey string             `json:"idempotency_key"`
	Items          []InspectionResult `json:"items"`
	CreatedAt      time.Time          `json:"created_at"`
}

type InspectionResult struct {
	ItemID         string   `json:"item_id"`
	RiskCategoryID string   `json:"risk_category_id"`
	Passed         bool     `json:"passed"`
	Severity       int      `json:"severity"`
	Likelihood     int      `json:"likelihood"`
	ImpactScopes   []string `json:"impact_scopes"`
	Notes          string   `json:"notes"`
}

func (i Inspection) ValidateNew() error {
	violations := make([]FieldViolation, 0)
	if i.SiteID == "" {
		violations = append(violations, FieldViolation{Field: "site_id", Message: "场地不能为空"})
	}
	if i.FacilityID == "" {
		violations = append(violations, FieldViolation{Field: "facility_id", Message: "设施不能为空"})
	}
	if i.IdempotencyKey == "" {
		violations = append(violations, FieldViolation{Field: "idempotency_key", Message: "幂等键不能为空"})
	}
	if len(i.Items) == 0 {
		violations = append(violations, FieldViolation{Field: "items", Message: "巡检至少包含一个巡检项"})
	}
	for position, result := range i.Items {
		violations = append(violations, result.violations(position)...)
	}
	if len(violations) > 0 {
		return ValidationError{Violations: violations}
	}
	return nil
}

func (r InspectionResult) violations(position int) []FieldViolation {
	prefix := fmt.Sprintf("items[%d]", position)
	violations := make([]FieldViolation, 0, 4)
	if r.ItemID == "" {
		violations = append(violations, FieldViolation{Field: prefix + ".item_id", Message: "巡检项不能为空"})
	}
	if r.RiskCategoryID == "" {
		violations = append(violations, FieldViolation{Field: prefix + ".risk_category_id", Message: "风险分类不能为空"})
	}
	if r.Severity < 1 || r.Severity > 5 {
		violations = append(violations, FieldViolation{Field: prefix + ".severity", Message: "严重程度必须在 1 到 5 之间"})
	}
	if r.Likelihood < 1 || r.Likelihood > 5 {
		violations = append(violations, FieldViolation{Field: prefix + ".likelihood", Message: "发生可能性必须在 1 到 5 之间"})
	}
	return violations
}

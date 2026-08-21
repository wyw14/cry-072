package domain

import (
	"strings"
	"time"
)

type RuleCondition struct {
	MinimumScore       int       `json:"minimum_score"`
	MinimumLevel       RiskLevel `json:"minimum_level"`
	RiskCategoryIDs    []string  `json:"risk_category_ids"`
	RequiredImpactTags []string  `json:"required_impact_tags"`
	MinimumEvidence    int       `json:"minimum_evidence"`
	CriticalFacility   *bool     `json:"critical_facility,omitempty"`
}

type EscalationRule struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Priority      int           `json:"priority"`
	Enabled       bool          `json:"enabled"`
	Version       int64         `json:"version"`
	Condition     RuleCondition `json:"condition"`
	FocusSLA      time.Duration `json:"focus_sla"`
	NotifyAfter   time.Duration `json:"notify_after"`
	ReNotifyEvery time.Duration `json:"renotify_every"`
	CreatedBy     string        `json:"created_by"`
	CreatedAt     time.Time     `json:"created_at"`
	UpdatedAt     time.Time     `json:"updated_at"`
}

func (r EscalationRule) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return NewValidationError("name", "升级规则名称不能为空")
	}
	if r.Priority < 1 || r.Priority > 1000 {
		return NewValidationError("priority", "规则优先级必须在 1 到 1000 之间")
	}
	if r.Condition.MinimumScore < 0 || r.Condition.MinimumScore > 100 {
		return NewValidationError("minimum_score", "最低风险分必须在 0 到 100 之间")
	}
	if r.Condition.MinimumLevel != "" && !r.Condition.MinimumLevel.Valid() {
		return NewValidationError("minimum_level", "最低风险等级无效")
	}
	if r.FocusSLA <= 0 || r.NotifyAfter < 0 || r.ReNotifyEvery <= 0 {
		return NewValidationError("schedule", "重点时限与提醒周期无效")
	}
	return nil
}

type RuleMatch struct {
	RuleID      string    `json:"rule_id"`
	RuleVersion int64     `json:"rule_version"`
	RuleName    string    `json:"rule_name"`
	MatchedAt   time.Time `json:"matched_at"`
	Reasons     []string  `json:"reasons"`
}

type DowngradeDecision struct {
	HazardID        string    `json:"hazard_id"`
	FromQueue       QueueKind `json:"from_queue"`
	ToQueue         QueueKind `json:"to_queue"`
	Reason          string    `json:"reason"`
	PreviousRuleID  string    `json:"previous_rule_id"`
	PreviousVersion int64     `json:"previous_rule_version"`
	DecidedBy       string    `json:"decided_by"`
	DecidedAt       time.Time `json:"decided_at"`
}

package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
)

type RuleInput struct {
	Hazard           domain.Hazard
	EvidenceCount    int
	CriticalFacility bool
}

type RuleEngine struct{}

func (RuleEngine) Evaluate(input RuleInput, rules []domain.EscalationRule, now time.Time) (*domain.RuleMatch, error) {
	ordered := append([]domain.EscalationRule(nil), rules...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].Priority == ordered[j].Priority {
			return ordered[i].Version > ordered[j].Version
		}
		return ordered[i].Priority < ordered[j].Priority
	})
	for _, rule := range ordered {
		if !rule.Enabled {
			continue
		}
		if err := rule.Validate(); err != nil {
			return nil, fmt.Errorf("rule %s invalid: %w", rule.ID, err)
		}
		reasons, matched := matchRule(input, rule)
		if !matched {
			continue
		}
		return &domain.RuleMatch{
			RuleID:      rule.ID,
			RuleVersion: rule.Version,
			RuleName:    rule.Name,
			MatchedAt:   now.UTC(),
			Reasons:     reasons,
		}, nil
	}
	return nil, nil
}

func matchRule(input RuleInput, rule domain.EscalationRule) ([]string, bool) {
	condition := rule.Condition
	reasons := make([]string, 0, 6)
	if condition.MinimumScore > 0 {
		if input.Hazard.RiskScore < condition.MinimumScore {
			return nil, false
		}
		reasons = append(reasons, fmt.Sprintf("risk_score:%d>=%d", input.Hazard.RiskScore, condition.MinimumScore))
	}
	if condition.MinimumLevel != "" {
		if input.Hazard.RiskLevel.Rank() < condition.MinimumLevel.Rank() {
			return nil, false
		}
		reasons = append(reasons, fmt.Sprintf("risk_level:%s>=%s", input.Hazard.RiskLevel, condition.MinimumLevel))
	}
	if len(condition.RiskCategoryIDs) > 0 {
		if !contains(condition.RiskCategoryIDs, input.Hazard.RiskCategoryID) {
			return nil, false
		}
		reasons = append(reasons, "risk_category:"+input.Hazard.RiskCategoryID)
	}
	if len(condition.RequiredImpactTags) > 0 {
		missing := missingValues(condition.RequiredImpactTags, input.Hazard.ImpactScopes)
		if len(missing) > 0 {
			return nil, false
		}
		for _, tag := range condition.RequiredImpactTags {
			reasons = append(reasons, "impact_scope:"+tag)
		}
	}
	if condition.MinimumEvidence > 0 {
		if input.EvidenceCount < condition.MinimumEvidence {
			return nil, false
		}
		reasons = append(reasons, fmt.Sprintf("evidence_count:%d>=%d", input.EvidenceCount, condition.MinimumEvidence))
	}
	if condition.CriticalFacility != nil {
		if input.CriticalFacility != *condition.CriticalFacility {
			return nil, false
		}
		reasons = append(reasons, fmt.Sprintf("critical_facility:%t", input.CriticalFacility))
	}
	if len(reasons) == 0 {
		return nil, false
	}
	return reasons, true
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}

func missingValues(required, actual []string) []string {
	set := make(map[string]struct{}, len(actual))
	for _, value := range actual {
		set[value] = struct{}{}
	}
	missing := make([]string, 0)
	for _, value := range required {
		if _, ok := set[value]; !ok {
			missing = append(missing, value)
		}
	}
	return missing
}

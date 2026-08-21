package service

import (
	"math"
	"sort"

	"github.com/wyw14/cry-072/internal/domain"
)

type RiskAssessment struct {
	Score   int
	Level   domain.RiskLevel
	Factors []string
}

type RiskCalculator struct{}

func (RiskCalculator) Calculate(category domain.RiskCategory, severity, likelihood int, impactScopes []string, criticalFacility bool) (RiskAssessment, error) {
	if severity < 1 || severity > 5 {
		return RiskAssessment{}, domain.NewValidationError("severity", "严重程度必须在 1 到 5 之间")
	}
	if likelihood < 1 || likelihood > 5 {
		return RiskAssessment{}, domain.NewValidationError("likelihood", "发生可能性必须在 1 到 5 之间")
	}
	if err := category.Validate(); err != nil {
		return RiskAssessment{}, err
	}

	base := severity * likelihood * 3
	categoryContribution := int(math.Round(float64(category.BaseWeight) * 0.25))
	impactContribution := min(len(uniqueNonEmpty(impactScopes))*4, 16)
	criticalContribution := 0
	if criticalFacility {
		criticalContribution = 12
	}
	score := min(base+categoryContribution+impactContribution+criticalContribution, 100)
	factors := []string{
		"severity_likelihood",
		"category_weight",
	}
	if impactContribution > 0 {
		factors = append(factors, "impact_scope")
	}
	if criticalFacility {
		factors = append(factors, "critical_facility")
	}
	return RiskAssessment{Score: score, Level: levelForScore(score), Factors: factors}, nil
}

func levelForScore(score int) domain.RiskLevel {
	switch {
	case score >= 80:
		return domain.RiskCritical
	case score >= 55:
		return domain.RiskHigh
	case score >= 30:
		return domain.RiskMedium
	default:
		return domain.RiskLow
	}
}

func uniqueNonEmpty(values []string) []string {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value != "" {
			set[value] = struct{}{}
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

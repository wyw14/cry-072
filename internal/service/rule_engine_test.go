package service

import (
	"testing"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
)

func TestRuleEngineRequiresEveryConfiguredCondition(t *testing.T) {
	critical := true
	rule := domain.EscalationRule{
		ID: "r1", Name: "关键设施规则", Priority: 1, Enabled: true, Version: 3,
		Condition: domain.RuleCondition{MinimumScore: 60, MinimumLevel: domain.RiskHigh, RiskCategoryIDs: []string{"electrical"}, RequiredImpactTags: []string{"people", "production"}, MinimumEvidence: 2, CriticalFacility: &critical},
		FocusSLA:  time.Hour, ReNotifyEvery: time.Hour,
	}
	input := RuleInput{Hazard: domain.Hazard{RiskScore: 80, RiskLevel: domain.RiskCritical, RiskCategoryID: "electrical", ImpactScopes: []string{"people", "production"}}, EvidenceCount: 2, CriticalFacility: true}
	match, err := (RuleEngine{}).Evaluate(input, []domain.EscalationRule{rule}, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.RuleVersion != 3 || len(match.Reasons) != 7 {
		t.Fatalf("unexpected match: %#v", match)
	}
	input.Hazard.ImpactScopes = []string{"people"}
	match, err = (RuleEngine{}).Evaluate(input, []domain.EscalationRule{rule}, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if match != nil {
		t.Fatalf("rule should not match with missing impact scope: %#v", match)
	}
}

func TestRuleEngineUsesPriorityThenLatestVersion(t *testing.T) {
	base := domain.EscalationRule{Name: "score", Enabled: true, Condition: domain.RuleCondition{MinimumScore: 10}, FocusSLA: time.Hour, ReNotifyEvery: time.Hour}
	first := base
	first.ID = "late"
	first.Priority = 20
	first.Version = 1
	second := base
	second.ID = "early"
	second.Priority = 10
	second.Version = 4
	match, err := (RuleEngine{}).Evaluate(RuleInput{Hazard: domain.Hazard{RiskScore: 20}}, []domain.EscalationRule{first, second}, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if match == nil || match.RuleID != "early" {
		t.Fatalf("unexpected priority match: %#v", match)
	}
}

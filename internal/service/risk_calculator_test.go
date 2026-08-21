package service

import (
	"testing"

	"github.com/wyw14/cry-072/internal/domain"
)

func TestRiskCalculatorAccountsForFacilityAndImpact(t *testing.T) {
	calculator := RiskCalculator{}
	category := domain.RiskCategory{Code: "ELEC", Name: "电气", BaseWeight: 60, DefaultSLA: 60}
	ordinary, err := calculator.Calculate(category, 3, 3, []string{"people"}, false)
	if err != nil {
		t.Fatal(err)
	}
	critical, err := calculator.Calculate(category, 3, 3, []string{"people", "production", "people"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if critical.Score <= ordinary.Score {
		t.Fatalf("critical score %d should exceed ordinary score %d", critical.Score, ordinary.Score)
	}
	if critical.Level.Rank() < ordinary.Level.Rank() {
		t.Fatalf("critical level %s should not be lower than %s", critical.Level, ordinary.Level)
	}
	if len(critical.Factors) != 4 {
		t.Fatalf("unexpected factors: %#v", critical.Factors)
	}
}

func TestRiskCalculatorRejectsInvalidScale(t *testing.T) {
	_, err := (RiskCalculator{}).Calculate(domain.RiskCategory{Code: "FIRE", Name: "消防", BaseWeight: 50, DefaultSLA: 30}, 0, 3, nil, false)
	if err == nil {
		t.Fatal("expected invalid severity error")
	}
}

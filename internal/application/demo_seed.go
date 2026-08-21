package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
)

func SeedDemo(ctx context.Context, repository Repository, now time.Time) error {
	critical := true
	values := []func(Store) error{
		func(store Store) error {
			return store.CreateSite(ctx, domain.Site{
				ID: "site_demo", Code: "SITE-DEMO", Name: "滨江演示场地", Region: "华东",
				State: domain.SiteActive, Version: 1, CreatedAt: now, UpdatedAt: now,
			})
		},
		func(store Store) error {
			return store.CreateFacility(ctx, domain.Facility{
				ID: "facility_demo", SiteID: "site_demo", Code: "POWER-01", Name: "一号配电设施",
				Kind: "electrical", Critical: true, Enabled: true, Version: 1, CreatedAt: now, UpdatedAt: now,
			})
		},
		func(store Store) error {
			return store.CreateRiskCategory(ctx, domain.RiskCategory{
				ID: "risk_electrical", Code: "ELECTRICAL", Name: "电气安全", BaseWeight: 72,
				DefaultSLA: 1440, Description: "配电、漏电、短路与过载风险", Version: 1, CreatedAt: now,
			})
		},
		func(store Store) error {
			return store.SaveRule(ctx, domain.EscalationRule{
				ID: "rule_critical_electrical", Name: "关键设施高风险自动升级", Description: "关键电气设施出现高风险时进入重点队列",
				Priority: 10, Enabled: true, Version: 1,
				Condition: domain.RuleCondition{MinimumScore: 55, MinimumLevel: domain.RiskHigh, RiskCategoryIDs: []string{"risk_electrical"}, CriticalFacility: &critical},
				FocusSLA:  8 * time.Hour, NotifyAfter: 15 * time.Minute, ReNotifyEvery: time.Hour,
				CreatedBy: "system-seed", CreatedAt: now, UpdatedAt: now,
			}, 0)
		},
	}
	for _, write := range values {
		if err := repository.WithinTx(ctx, write); err != nil && !errors.Is(err, domain.ErrDuplicate) {
			return fmt.Errorf("seed demo data: %w", err)
		}
	}
	return nil
}

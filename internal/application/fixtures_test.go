package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
	"github.com/wyw14/cry-072/internal/repository/memory"
)

type fixture struct {
	repository *memory.Repository
	clock      *platform.ManualTimeSource
	ids        *platform.SequenceIDGenerator
	meta       application.RequestMeta
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	ctx := context.Background()
	repository := memory.New()
	clock := platform.NewManualTimeSource(time.Date(2026, 8, 22, 9, 0, 0, 0, time.UTC))
	ids := &platform.SequenceIDGenerator{}
	if err := repository.CreateSite(ctx, domain.Site{ID: "site-1", Code: "S1", Name: "演示场地", Region: "华东", State: domain.SiteActive, Version: 1, CreatedAt: clock.Now(), UpdatedAt: clock.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateFacility(ctx, domain.Facility{ID: "facility-1", SiteID: "site-1", Code: "F1", Name: "关键配电柜", Kind: "electrical", Critical: true, Enabled: true, Version: 1, CreatedAt: clock.Now(), UpdatedAt: clock.Now()}); err != nil {
		t.Fatal(err)
	}
	if err := repository.CreateRiskCategory(ctx, domain.RiskCategory{ID: "risk-1", Code: "ELEC", Name: "电气安全", BaseWeight: 80, DefaultSLA: 720, Version: 1, CreatedAt: clock.Now()}); err != nil {
		t.Fatal(err)
	}
	critical := true
	if err := repository.SaveRule(ctx, domain.EscalationRule{ID: "rule-1", Name: "重点升级", Priority: 1, Enabled: true, Version: 1, Condition: domain.RuleCondition{MinimumScore: 55, MinimumLevel: domain.RiskHigh, RiskCategoryIDs: []string{"risk-1"}, MinimumEvidence: 1, CriticalFacility: &critical}, FocusSLA: 4 * time.Hour, NotifyAfter: 10 * time.Minute, ReNotifyEvery: time.Hour, CreatedAt: clock.Now(), UpdatedAt: clock.Now()}, 0); err != nil {
		t.Fatal(err)
	}
	return fixture{repository: repository, clock: clock, ids: ids, meta: application.RequestMeta{ActorID: "supervisor", Role: domain.RoleSupervisor, RequestID: "req-1"}}
}

func (f fixture) reportInput(key string) application.ReportHazardInput {
	return application.ReportHazardInput{SiteID: "site-1", FacilityID: "facility-1", InspectionItemID: "item-1", RiskCategoryID: "risk-1", Title: "配电柜温度异常", Description: "红外测温发现接头持续升温", Severity: 5, Likelihood: 4, ImpactScopes: []string{"people", "production"}, InitialAction: "切断支路并设置警戒", IdempotencyKey: key, Evidence: []application.EvidenceInput{{Kind: "image", FileName: "thermal.png", ContentType: "image/png", StorageKey: "hash.png", SizeBytes: 128}}}
}

package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/repository/postgres"
)

func TestPostgresOptimisticVersionAndRelationalRoundTrip(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not configured")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	migration, err := os.ReadFile(filepath.Join("..", "migrations", "001_initial.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	repository, err := postgres.Open(ctx, databaseURL, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer repository.Close()
	now := time.Now().UTC()
	site := domain.Site{ID: "integration-site", Code: "INTEGRATION", Name: "集成测试场地", Region: "test", State: domain.SiteActive, Version: 1, CreatedAt: now, UpdatedAt: now}
	_, _ = pool.Exec(ctx, `DELETE FROM sites WHERE id=$1`, site.ID)
	if err := repository.CreateSite(ctx, site); err != nil {
		t.Fatal(err)
	}
	loaded, err := repository.GetSite(ctx, site.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Name != site.Name {
		t.Fatalf("round trip mismatch: %#v", loaded)
	}
	loaded.State = domain.SiteIsolated
	loaded.Version = 2
	if err := repository.UpdateSite(ctx, loaded, 1); err != nil {
		t.Fatal(err)
	}
	if err := repository.UpdateSite(ctx, loaded, 1); err != domain.ErrConflict {
		t.Fatalf("expected conflict, got %v", err)
	}
	critical := true
	rule := domain.EscalationRule{
		ID: "integration-rule", Name: "关键设施高风险升级", Description: "验证关系化规则版本存储",
		Priority: 8, Enabled: true, Version: 1,
		Condition: domain.RuleCondition{
			MinimumScore: 80, MinimumLevel: domain.RiskHigh,
			RiskCategoryIDs: []string{"risk_electrical"}, RequiredImpactTags: []string{"people"},
			MinimumEvidence: 1, CriticalFacility: &critical,
		},
		FocusSLA: 90 * time.Minute, NotifyAfter: 15 * time.Minute, ReNotifyEvery: 30 * time.Minute,
		CreatedBy: "integration", CreatedAt: now, UpdatedAt: now,
	}
	_, _ = pool.Exec(ctx, `DELETE FROM escalation_rule_versions WHERE rule_id=$1`, rule.ID)
	if err := repository.SaveRule(ctx, rule, 0); err != nil {
		t.Fatal(err)
	}
	loadedRule, err := repository.GetRule(ctx, rule.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedRule.FocusSLA != rule.FocusSLA || loadedRule.ReNotifyEvery != rule.ReNotifyEvery {
		t.Fatalf("rule schedule mismatch: %#v", loadedRule)
	}
	if len(loadedRule.Condition.RiskCategoryIDs) != 1 || loadedRule.Condition.CriticalFacility == nil || !*loadedRule.Condition.CriticalFacility {
		t.Fatalf("rule condition mismatch: %#v", loadedRule.Condition)
	}
}

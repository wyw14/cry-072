package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
)

func TestAnalyticsKeepsNormalAndFocusDenominatorsSeparate(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	now := f.clock.Now()
	rows := []domain.Hazard{
		{ID: "n1", IdempotencyKey: "n1", SiteID: "site-1", Queue: domain.QueueNormal, State: domain.StateResolved, RiskLevel: domain.RiskMedium, RiskScore: 40, CreatedAt: now.Add(-4 * time.Hour), UpdatedAt: now.Add(-2 * time.Hour), DueAt: now.Add(-time.Hour), Version: 1},
		{ID: "f1", IdempotencyKey: "f1", SiteID: "site-1", Queue: domain.QueueFocus, State: domain.StateResolved, RiskLevel: domain.RiskHigh, RiskScore: 70, CreatedAt: now.Add(-5 * time.Hour), UpdatedAt: now.Add(-4 * time.Hour), DueAt: now.Add(-3 * time.Hour), Version: 1},
		{ID: "f2", IdempotencyKey: "f2", SiteID: "site-1", Queue: domain.QueueFocus, State: domain.StateHandling, RiskLevel: domain.RiskCritical, RiskScore: 90, CreatedAt: now.Add(-time.Hour), UpdatedAt: now, DueAt: now.Add(-time.Minute), Version: 1},
	}
	for _, row := range rows {
		if err := f.repository.SaveHazard(ctx, row); err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := application.NewAnalyticsService(f.repository, f.clock).Snapshot(ctx, now.Add(-24*time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Normal.ResolvedTotal != 1 || snapshot.Normal.OnTimeRate != 1 || snapshot.Normal.OpenCount != 1 {
		t.Fatalf("unexpected normal metrics: %#v", snapshot.Normal)
	}
	if snapshot.Focus.ResolvedTotal != 1 || snapshot.Focus.OnTimeRate != 1 || snapshot.Focus.OpenCount != 2 {
		t.Fatalf("unexpected focus metrics: %#v", snapshot.Focus)
	}
}

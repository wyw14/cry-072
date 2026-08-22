package application_test

import (
	"context"
	"testing"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
)

func TestReinspectionDoesNotRestoreSiteWhileAnotherFocusHazardIsResolvedButNotClosed(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	hazardService := application.NewHazardService(f.repository, f.clock, f.ids)
	first, err := hazardService.Report(ctx, f.reportInput("first"), f.meta)
	if err != nil {
		t.Fatal(err)
	}
	secondInput := f.reportInput("second")
	secondInput.Title = "另一处已复检通过但未关闭的重点隐患"
	second, err := hazardService.Report(ctx, secondInput, f.meta)
	if err != nil {
		t.Fatal(err)
	}

	// second has passed reinspection but is still awaiting explicit closure.
	second.State = domain.StateResolved
	second.PassedReinspectionAt = ptr(f.clock.Now())
	second.Version++
	second.UpdatedAt = f.clock.Now()
	if err := f.repository.UpdateHazard(ctx, second, second.Version-1); err != nil {
		t.Fatal(err)
	}

	// first reaches the reviewable state and is about to be reinspected.
	first.State = domain.StateReview
	first.Version++
	first.UpdatedAt = f.clock.Now()
	if err := f.repository.UpdateHazard(ctx, first, first.Version-1); err != nil {
		t.Fatal(err)
	}

	site, _ := f.repository.GetSite(ctx, "site-1")
	site.State = domain.SiteIsolated
	site.Version++
	site.UpdatedAt = f.clock.Now()
	if err := f.repository.UpdateSite(ctx, site, site.Version-1); err != nil {
		t.Fatal(err)
	}

	service := application.NewRemediationService(f.repository, f.clock, f.ids)
	updated, err := service.Reinspect(ctx, domain.Reinspection{
		HazardID: first.ID, Passed: true, Notes: "整改有效，现场指标恢复",
		EvidenceIDs: []string{"e1"}, RestoreSite: true, ObservedScore: 10, IdempotencyKey: "reinspect-resolved",
	}, first.Version, application.RequestMeta{ActorID: "reviewer", Role: domain.RoleReviewer, RequestID: "r2"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != domain.StateResolved || updated.PassedReinspectionAt == nil {
		t.Fatalf("unexpected reinspection result: %#v", updated)
	}

	// second is only resolved, not closed: the site must stay isolated until it
	// is genuinely closed.
	site, _ = f.repository.GetSite(ctx, "site-1")
	if site.State != domain.SiteIsolated {
		t.Fatalf("site restored while another focus hazard was resolved but not closed: %s", site.State)
	}
}

func ptr(t time.Time) *time.Time { return &t }

func TestReinspectionDoesNotRestoreSiteWhileAnotherFocusHazardIsOpen(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	hazardService := application.NewHazardService(f.repository, f.clock, f.ids)
	first, err := hazardService.Report(ctx, f.reportInput("first"), f.meta)
	if err != nil {
		t.Fatal(err)
	}
	secondInput := f.reportInput("second")
	secondInput.Title = "另一处关键隐患"
	if _, err := hazardService.Report(ctx, secondInput, f.meta); err != nil {
		t.Fatal(err)
	}
	first.State = domain.StateReview
	first.Version++
	first.UpdatedAt = f.clock.Now()
	if err := f.repository.UpdateHazard(ctx, first, first.Version-1); err != nil {
		t.Fatal(err)
	}
	site, _ := f.repository.GetSite(ctx, "site-1")
	site.State = domain.SiteIsolated
	site.Version++
	site.UpdatedAt = f.clock.Now()
	if err := f.repository.UpdateSite(ctx, site, site.Version-1); err != nil {
		t.Fatal(err)
	}
	service := application.NewRemediationService(f.repository, f.clock, f.ids)
	updated, err := service.Reinspect(ctx, domain.Reinspection{HazardID: first.ID, Passed: true, Notes: "整改有效，现场指标恢复", EvidenceIDs: []string{"e1"}, RestoreSite: true, ObservedScore: 10, IdempotencyKey: "reinspect-1"}, first.Version, application.RequestMeta{ActorID: "reviewer", Role: domain.RoleReviewer, RequestID: "r2"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != domain.StateResolved || updated.PassedReinspectionAt == nil {
		t.Fatalf("unexpected reinspection result: %#v", updated)
	}
	site, _ = f.repository.GetSite(ctx, "site-1")
	if site.State != domain.SiteIsolated {
		t.Fatalf("site restored while another focus hazard remained: %s", site.State)
	}
}

func TestPlanIsolationAndTaskCompletion(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	hazard, err := application.NewHazardService(f.repository, f.clock, f.ids).Report(ctx, f.reportInput("plan"), f.meta)
	if err != nil {
		t.Fatal(err)
	}
	service := application.NewRemediationService(f.repository, f.clock, f.ids)
	plan, err := service.SavePlan(ctx, domain.RemediationPlan{HazardID: hazard.ID, Summary: "更换接头并复测", TemporaryBlock: true, Tasks: []domain.RemediationTask{{Title: "更换接头", OwnerID: "tech", DueAt: f.clock.Now().Add(time.Hour)}}}, 0, f.meta)
	if err != nil {
		t.Fatal(err)
	}
	site, _ := f.repository.GetSite(ctx, "site-1")
	if site.State != domain.SiteIsolated {
		t.Fatalf("site not isolated: %s", site.State)
	}
	plan, err = service.CompleteTask(ctx, hazard.ID, plan.Tasks[0].ID, []string{"after-photo"}, plan.Version, f.meta)
	if err != nil {
		t.Fatal(err)
	}
	if !plan.Complete() {
		t.Fatal("plan should be complete")
	}
}

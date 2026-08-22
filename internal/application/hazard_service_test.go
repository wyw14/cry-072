package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
)

func TestReportHazardCreatesFocusQueueAndRuleEvidenceAtomically(t *testing.T) {
	f := newFixture(t)
	service := application.NewHazardService(f.repository, f.clock, f.ids)
	hazard, err := service.Report(context.Background(), f.reportInput("report-1"), f.meta)
	if err != nil {
		t.Fatal(err)
	}
	if hazard.Queue != domain.QueueFocus || !hazard.Escalated {
		t.Fatalf("expected focus escalation: %#v", hazard)
	}
	if hazard.MatchedRuleID != "rule-1" || len(hazard.MatchedRuleEvidence) < 4 {
		t.Fatalf("missing rule evidence: %#v", hazard.MatchedRuleEvidence)
	}
	queue, err := f.repository.GetQueueEntry(context.Background(), hazard.ID)
	if err != nil {
		t.Fatal(err)
	}
	if queue.Queue != domain.QueueFocus || queue.NextNotifyAt.IsZero() {
		t.Fatalf("unexpected queue: %#v", queue)
	}
	evidence, err := f.repository.ListEvidence(context.Background(), hazard.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].StorageKey != "hash.png" {
		t.Fatalf("unexpected evidence: %#v", evidence)
	}
}

func TestReportHazardIdempotencyReturnsOriginalAggregate(t *testing.T) {
	f := newFixture(t)
	service := application.NewHazardService(f.repository, f.clock, f.ids)
	first, err := service.Report(context.Background(), f.reportInput("same-key"), f.meta)
	if err != nil {
		t.Fatal(err)
	}
	secondInput := f.reportInput("same-key")
	secondInput.Title = "不同标题"
	second, err := service.Report(context.Background(), secondInput, f.meta)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || second.Title != first.Title {
		t.Fatalf("idempotency changed aggregate: %#v %#v", first, second)
	}
}

func TestReportKeepsSameNameDistinctStorageEvidence(t *testing.T) {
	f := newFixture(t)
	critical := true
	if err := f.repository.SaveRule(context.Background(), domain.EscalationRule{
		ID: "rule-2", Name: "双证据升级", Priority: 1, Enabled: true, Version: 1,
		Condition: domain.RuleCondition{
			MinimumScore: 55, MinimumLevel: domain.RiskHigh, RiskCategoryIDs: []string{"risk-1"},
			MinimumEvidence: 2, CriticalFacility: &critical,
		},
		FocusSLA: 4 * time.Hour, NotifyAfter: 10 * time.Minute, ReNotifyEvery: time.Hour,
		CreatedAt: f.clock.Now(), UpdatedAt: f.clock.Now(),
	}, 0); err != nil {
		t.Fatal(err)
	}
	// Disable the single-evidence rule so rule-2 is the only candidate.
	if rule, err := f.repository.GetRule(context.Background(), "rule-1"); err == nil {
		rule.Enabled = false
		if err := f.repository.SaveRule(context.Background(), rule, rule.Version); err != nil {
			t.Fatal(err)
		}
	}
	service := application.NewHazardService(f.repository, f.clock, f.ids)

	input := f.reportInput("evidence-distinct")
	// Two photos from different points sharing an original file name but
	// resolving to distinct content-addressed storage objects.
	input.Evidence = []application.EvidenceInput{
		{Kind: "image", FileName: "point.jpg", ContentType: "image/png", StorageKey: "hash-a.png", SizeBytes: 128},
		{Kind: "image", FileName: "point.jpg", ContentType: "image/png", StorageKey: "hash-b.png", SizeBytes: 256},
	}
	hazard, err := service.Report(context.Background(), input, f.meta)
	if err != nil {
		t.Fatal(err)
	}
	if !hazard.Escalated || hazard.MatchedRuleID != "rule-2" {
		t.Fatalf("expected escalation on two distinct evidence, got %#v", hazard)
	}
	evidence, err := f.repository.ListEvidence(context.Background(), hazard.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 2 {
		t.Fatalf("expected both same-name distinct-storage attachments retained, got %d: %#v", len(evidence), evidence)
	}
	keys := map[string]bool{evidence[0].StorageKey: true, evidence[1].StorageKey: true}
	if !keys["hash-a.png"] || !keys["hash-b.png"] {
		t.Fatalf("expected hash-a.png and hash-b.png retained, got %#v", evidence)
	}
}

func TestReportCollapsesSameStorageRetryEvidence(t *testing.T) {
	f := newFixture(t)
	service := application.NewHazardService(f.repository, f.clock, f.ids)

	input := f.reportInput("evidence-retry")
	// Two retries of the same photo: identical storage object, same file name.
	// The second carries a richer description and must merge, not duplicate.
	input.Evidence = []application.EvidenceInput{
		{Kind: "image", FileName: "thermal.png", ContentType: "image/png", StorageKey: "hash.png", SizeBytes: 128, Description: "东侧接头"},
		{Kind: "image", FileName: "thermal.png", ContentType: "image/png", StorageKey: "hash.png", SizeBytes: 128, Description: "复核确认"},
	}
	hazard, err := service.Report(context.Background(), input, f.meta)
	if err != nil {
		t.Fatal(err)
	}
	evidence, err := f.repository.ListEvidence(context.Background(), hazard.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 {
		t.Fatalf("expected retries of one storage object collapsed, got %d: %#v", len(evidence), evidence)
	}
	if evidence[0].StorageKey != "hash.png" || !strings.Contains(evidence[0].Description, "复核确认") {
		t.Fatalf("unexpected merged evidence: %#v", evidence[0])
	}
}

func TestAssignmentUpdatesHazardQueueAndAuditInOneTransaction(t *testing.T) {
	f := newFixture(t)
	service := application.NewHazardService(f.repository, f.clock, f.ids)
	hazard, err := service.Report(context.Background(), f.reportInput("assign-key"), f.meta)
	if err != nil {
		t.Fatal(err)
	}
	updated, err := service.AssignOwner(context.Background(), hazard.ID, "owner-7", hazard.Version, f.meta)
	if err != nil {
		t.Fatal(err)
	}
	queue, err := f.repository.GetQueueEntry(context.Background(), hazard.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.OwnerID != "owner-7" || queue.OwnerID != "owner-7" {
		t.Fatalf("assignment mismatch: %#v %#v", updated, queue)
	}
	audit, err := f.repository.ListAudit(context.Background(), hazard.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if audit.Total != 2 {
		t.Fatalf("expected report and assignment audit, got %d", audit.Total)
	}
}

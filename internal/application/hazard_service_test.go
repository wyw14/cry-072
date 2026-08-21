package application_test

import (
	"context"
	"testing"

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

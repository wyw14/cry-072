package application_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
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

// TestConcurrentAssignmentAllowsOnlyOneWinner guards against the stale-overwrite
// bug where two supervisors reading the same version both "succeed": the loser's
// retry used to reload the latest version, skip the version precheck and overwrite
// the winner. Only one assignment may take effect and the stale caller must see a
// conflict rather than silently clobbering the concurrent winner.
func TestConcurrentAssignmentAllowsOnlyOneWinner(t *testing.T) {
	f := newFixture(t)
	service := application.NewHazardService(f.repository, f.clock, f.ids)
	hazard, err := service.Report(context.Background(), f.reportInput("concurrent-assign"), f.meta)
	if err != nil {
		t.Fatal(err)
	}
	owners := []string{"owner-alpha", "owner-beta"}
	version := hazard.Version

	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(len(owners))
	var done sync.WaitGroup
	done.Add(len(owners))
	type outcome struct {
		owner string
		err   error
	}
	outcomes := make([]outcome, len(owners))
	for index, owner := range owners {
		go func(index int, owner string) {
			defer done.Done()
			ready.Done()
			<-start
			_, err := service.AssignOwner(context.Background(), hazard.ID, owner, version, f.meta)
			outcomes[index] = outcome{owner: owner, err: err}
		}(index, owner)
	}
	ready.Wait()
	close(start)
	done.Wait()

	var successes atomic.Int32
	var conflicts atomic.Int32
	var winner string
	for _, outcome := range outcomes {
		switch {
		case outcome.err == nil:
			successes.Add(1)
			winner = outcome.owner
		case errors.Is(outcome.err, domain.ErrConflict):
			conflicts.Add(1)
		default:
			t.Fatalf("unexpected assign error for %s: %v", outcome.owner, outcome.err)
		}
	}
	if successes.Load() != 1 || conflicts.Load() != 1 {
		t.Fatalf("expected exactly one success and one conflict, got successes=%d conflicts=%d", successes.Load(), conflicts.Load())
	}

	// The persisted owner must match the single winner, not the last writer.
	queue, err := f.repository.GetQueueEntry(context.Background(), hazard.ID)
	if err != nil {
		t.Fatal(err)
	}
	hazard2, err := f.repository.GetHazard(context.Background(), hazard.ID)
	if err != nil {
		t.Fatal(err)
	}
	if queue.OwnerID != winner || hazard2.OwnerID != winner {
		t.Fatalf("concurrent assignment overwrote the winner: queue=%q hazard=%q winner=%q", queue.OwnerID, hazard2.OwnerID, winner)
	}
	if hazard2.Version != version+1 {
		t.Fatalf("expected exactly one version bump to %d, got %d", version+1, hazard2.Version)
	}

	// Exactly one assignment audit event must have been appended (the report audit is the other).
	audit, err := f.repository.ListAudit(context.Background(), hazard.ID, 1, 20)
	if err != nil {
		t.Fatal(err)
	}
	if audit.Total != 2 {
		t.Fatalf("expected only report and one assignment audit, got %d", audit.Total)
	}
}


package application_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
)

// flakyAdapter fails its first Send call and succeeds afterwards. It records
// every notification it is asked to deliver so tests can assert whether later
// entries in a batch were reached after an earlier failure.
type flakyAdapter struct {
	mu          sync.Mutex
	failure     error
	deliveries  []string
	sendCalls   int
}

func newFlakyAdapter(failure error) *flakyAdapter {
	return &flakyAdapter{failure: failure}
}

func (a *flakyAdapter) Name() string { return "flaky-test" }

func (a *flakyAdapter) Send(ctx context.Context, notification domain.Notification) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sendCalls++
	if a.sendCalls == 1 && a.failure != nil {
		return a.failure
	}
	a.deliveries = append(a.deliveries, notification.HazardID)
	return nil
}

func (a *flakyAdapter) deliveredHazardIDs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	result := make([]string, len(a.deliveries))
	copy(result, a.deliveries)
	return result
}

// reportDueFocusHazard reports an escalated hazard and advances the clock past
// its first notification due time so it shows up in a due batch.
func reportDueFocusHazard(t *testing.T, f fixture, key string) domain.Hazard {
	t.Helper()
	ctx := context.Background()
	hazard, err := application.NewHazardService(f.repository, f.clock, f.ids).Report(ctx, f.reportInput(key), f.meta)
	if err != nil {
		t.Fatal(err)
	}
	if hazard.Queue != domain.QueueFocus {
		t.Fatalf("expected focus hazard for %s, got queue %s", key, hazard.Queue)
	}
	// rule-1 notifies after 10 minutes; step just past that window.
	f.clock.Advance(11 * time.Minute)
	return hazard
}

func TestProcessDuePropagatesFirstSendFailureAndStopsBatch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	first := reportDueFocusHazard(t, f, "due-first")
	second := reportDueFocusHazard(t, f, "due-second")
	if first.ID == second.ID {
		t.Fatalf("expected distinct hazards, both %s", first.ID)
	}

	adapter := newFlakyAdapter(errors.New("local adapter unavailable"))
	service := application.NewNotificationService(f.repository, f.clock, f.ids, adapter)

	processed, err := service.ProcessDue(ctx, 50)
	if err == nil {
		t.Fatal("expected first send failure to propagate as error, got nil")
	}
	if processed != 0 {
		t.Fatalf("expected zero processed before the failure, got %d", processed)
	}

	delivered := adapter.deliveredHazardIDs()
	if len(delivered) != 0 {
		t.Fatalf("expected no successful deliveries after first failure, got %v", delivered)
	}
	// The second reminder must never have been attempted: batch processing stops
	// at the first error instead of swallowing it and continuing.
	if calls := adapter.sendCalls; calls != 1 {
		t.Fatalf("expected exactly one send attempt before stopping, got %d", calls)
	}
}

func TestProcessDueReturnsNilAndSendsAllWhenBatchSucceeds(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	first := reportDueFocusHazard(t, f, "due-ok-first")
	second := reportDueFocusHazard(t, f, "due-ok-second")
	if first.ID == second.ID {
		t.Fatalf("expected distinct hazards, both %s", first.ID)
	}

	adapter := newFlakyAdapter(nil) // never fails
	service := application.NewNotificationService(f.repository, f.clock, f.ids, adapter)

	processed, err := service.ProcessDue(ctx, 50)
	if err != nil {
		t.Fatalf("expected no error for successful batch, got %v", err)
	}
	if processed != 2 {
		t.Fatalf("expected 2 processed reminders, got %d", processed)
	}
	delivered := adapter.deliveredHazardIDs()
	if len(delivered) != 2 {
		t.Fatalf("expected both reminders delivered, got %v", delivered)
	}
}

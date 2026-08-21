package memory

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
)

func TestConcurrentOptimisticUpdatesAllowOnlyOneWinner(t *testing.T) {
	repository := New()
	ctx := context.Background()
	now := time.Now()
	hazard := domain.Hazard{ID: "h1", IdempotencyKey: "key", SiteID: "s", Queue: domain.QueueNormal, State: domain.StatePending, RiskLevel: domain.RiskLow, Version: 1, CreatedAt: now, UpdatedAt: now}
	if err := repository.SaveHazard(ctx, hazard); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(2)
	var success atomic.Int32
	var conflicts atomic.Int32
	var done sync.WaitGroup
	done.Add(2)
	for index := 0; index < 2; index++ {
		go func(owner string) {
			defer done.Done()
			candidate := hazard
			candidate.OwnerID = owner
			candidate.Version = 2
			ready.Done()
			<-start
			if err := repository.UpdateHazard(ctx, candidate, 1); err == nil {
				success.Add(1)
			} else if err == domain.ErrConflict {
				conflicts.Add(1)
			} else {
				t.Errorf("unexpected update error: %v", err)
			}
		}(string(rune('a' + index)))
	}
	ready.Wait()
	close(start)
	done.Wait()
	if success.Load() != 1 || conflicts.Load() != 1 {
		t.Fatalf("success=%d conflicts=%d", success.Load(), conflicts.Load())
	}
}

func TestTransactionRollsBackAllWrites(t *testing.T) {
	repository := New()
	ctx := context.Background()
	now := time.Now()
	err := repository.WithinTx(ctx, func(store application.Store) error {
		if err := store.CreateSite(ctx, domain.Site{ID: "s", Code: "S", Name: "site", State: domain.SiteActive, Version: 1, CreatedAt: now, UpdatedAt: now}); err != nil {
			return err
		}
		return domain.ErrValidation
	})
	if err == nil {
		t.Fatal("expected rollback error")
	}
	if _, err := repository.GetSite(ctx, "s"); err != domain.ErrNotFound {
		t.Fatalf("site persisted after rollback: %v", err)
	}
}

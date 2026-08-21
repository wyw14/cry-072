package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
	"github.com/wyw14/cry-072/internal/service"
)

type NotificationService struct {
	repository Repository
	clock      platform.TimeSource
	ids        platform.IDGenerator
	adapter    platform.NotificationAdapter
	deadlines  service.DeadlinePolicy
}

func NewNotificationService(repository Repository, clock platform.TimeSource, ids platform.IDGenerator, adapter platform.NotificationAdapter) *NotificationService {
	return &NotificationService{repository: repository, clock: clock, ids: ids, adapter: adapter, deadlines: service.NewDeadlinePolicy()}
}

func (s *NotificationService) ProcessDue(ctx context.Context, limit int) (int, error) {
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	now := s.clock.Now()
	entries, err := s.repository.ListDueQueueEntries(ctx, now, limit)
	if err != nil {
		return 0, fmt.Errorf("list due queue entries: %w", err)
	}
	sent := 0
	for _, entry := range entries {
		if err := s.processEntry(ctx, entry, now); err != nil {
			return sent, err
		}
		sent++
	}
	return sent, nil
}

func (s *NotificationService) processEntry(ctx context.Context, entry domain.QueueEntry, now time.Time) error {
	hazard, err := s.repository.GetHazard(ctx, entry.HazardID)
	if err != nil {
		return fmt.Errorf("load notification hazard: %w", err)
	}
	if hazard.State == domain.StateClosed || !entry.AcknowledgedAt.IsZero() {
		return nil
	}
	tier := entry.EscalationTier
	if !entry.LastNotifiedAt.IsZero() {
		tier++
	}
	deduplication := hazard.ID + ":" + strconv.Itoa(tier)
	if _, err := s.repository.FindNotificationByDeduplication(ctx, deduplication); err == nil {
		return nil
	} else if !errors.Is(err, domain.ErrNotFound) {
		return err
	}
	recipient := entry.OwnerID
	if recipient == "" {
		recipient = "safety-supervisors"
	}
	notification := domain.Notification{
		ID:       s.ids.New("notice"),
		HazardID: hazard.ID,
		Destination: domain.NotificationDestination{
			OperatorID: recipient, Adapter: s.adapter.Name(),
		},
		Notice: domain.EscalationNotice{
			Template: "hazard_escalation", Tier: tier,
			Body: fmt.Sprintf("隐患 %s 已进入第 %d 级提醒，截止时间 %s", hazard.Title, tier, hazard.DueAt.Format(time.RFC3339)),
		},
		Delivery:  domain.DeliveryState{DeduplicationKey: deduplication},
		CreatedAt: now,
	}
	sendErr := s.adapter.Send(ctx, notification)
	attempt := domain.NotificationAttempt{NotificationID: notification.ID, Adapter: s.adapter.Name(), AttemptedAt: now}
	if sendErr != nil {
		attempt.ErrorCode = "LOCAL_ADAPTER_ERROR"
	} else {
		attempt.Succeeded = true
		notification.MarkSent(now)
	}
	err = s.repository.WithinTx(ctx, func(store Store) error {
		if err := store.SaveNotification(ctx, notification); err != nil {
			return err
		}
		if err := store.SaveNotificationAttempt(ctx, attempt); err != nil {
			return err
		}
		if sendErr == nil {
			entry.LastNotifiedAt = now
			entry.EscalationTier = tier
			entry.NextNotifyAt = s.deadlines.NextNotification(entry, nil, now)
			oldVersion := entry.Version
			entry.Version++
			entry.UpdatedAt = now
			if err := store.SaveQueueEntry(ctx, entry, oldVersion); err != nil {
				return err
			}
		}
		return store.AppendAudit(ctx, domain.AuditEvent{
			ID: s.ids.New("audit"), HazardID: hazard.ID, SiteID: hazard.SiteID,
			EventType: "notification.attempted", ActorID: "local-scheduler", ActorRole: domain.RoleAdmin,
			OccurredAt: now, Details: map[string]any{"tier": tier, "recipient": recipient, "succeeded": sendErr == nil},
			SensitiveKeys: []string{"recipient"},
		})
	})
	if err != nil {
		return fmt.Errorf("persist notification: %w", err)
	}
	if sendErr != nil {
		return fmt.Errorf("send local notification: %w", sendErr)
	}
	return nil
}

func (s *NotificationService) Acknowledge(ctx context.Context, hazardID string, expectedVersion int64, meta RequestMeta) error {
	entry, err := s.repository.GetQueueEntry(ctx, hazardID)
	if err != nil {
		return err
	}
	if entry.Version != expectedVersion {
		return domain.ErrConflict
	}
	if entry.OwnerID != "" && entry.OwnerID != meta.ActorID && !meta.Role.CanConfigureRules() {
		return domain.ErrForbidden
	}
	entry.AcknowledgedAt = s.clock.Now()
	entry.Version++
	entry.UpdatedAt = entry.AcknowledgedAt
	return s.repository.SaveQueueEntry(ctx, entry, expectedVersion)
}

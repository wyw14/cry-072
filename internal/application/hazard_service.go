package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
	"github.com/wyw14/cry-072/internal/service"
)

type HazardService struct {
	repository Repository
	clock      platform.TimeSource
	ids        platform.IDGenerator
	calculator service.RiskCalculator
	rules      service.RuleEngine
	deadlines  service.DeadlinePolicy
}

func NewHazardService(repository Repository, clock platform.TimeSource, ids platform.IDGenerator) *HazardService {
	return &HazardService{
		repository: repository, clock: clock, ids: ids,
		calculator: service.RiskCalculator{}, rules: service.RuleEngine{}, deadlines: service.NewDeadlinePolicy(),
	}
}

func (s *HazardService) CreateInspection(ctx context.Context, inspection domain.Inspection, meta RequestMeta) (domain.Inspection, error) {
	inspection.IdempotencyKey = strings.TrimSpace(inspection.IdempotencyKey)
	if err := inspection.ValidateNew(); err != nil {
		return domain.Inspection{}, err
	}
	facility, err := s.repository.GetFacility(ctx, inspection.FacilityID)
	if err != nil {
		return domain.Inspection{}, fmt.Errorf("load inspection facility: %w", err)
	}
	if facility.SiteID != inspection.SiteID || !facility.Enabled {
		return domain.Inspection{}, domain.NewValidationError("facility_id", "巡检设施不属于场地或已停用")
	}
	now := s.clock.Now()
	inspection.ID = s.ids.New("inspection")
	inspection.InspectorID = meta.ActorID
	inspection.StartedAt = inspection.StartedAt.UTC()
	if inspection.StartedAt.IsZero() {
		inspection.StartedAt = now
	}
	inspection.CompletedAt = now
	inspection.CreatedAt = now
	if err := s.repository.SaveInspection(ctx, inspection); err != nil {
		return domain.Inspection{}, fmt.Errorf("save inspection: %w", err)
	}
	return inspection, nil
}

func (s *HazardService) Get(ctx context.Context, id string) (domain.Hazard, error) {
	hazard, err := s.repository.GetHazard(ctx, id)
	if err != nil {
		return domain.Hazard{}, fmt.Errorf("get hazard: %w", err)
	}
	return hazard, nil
}

func (s *HazardService) ListQueue(ctx context.Context, filter domain.QueueFilter) (domain.Page[domain.Hazard], error) {
	return s.repository.ListQueue(ctx, filter.Normalize(), s.clock.Now())
}

type queueMutationAudit struct {
	reason        string
	ruleID        string
	ruleVersion   int64
	details       map[string]any
	sensitiveKeys []string
}

type queueMutation func(*domain.Hazard, *domain.QueueEntry, time.Time) (queueMutationAudit, error)

func (s *HazardService) mutateQueueCase(
	ctx context.Context,
	hazardID string,
	expectedVersion int64,
	eventType string,
	meta RequestMeta,
	change queueMutation,
) (domain.Hazard, error) {
	var changed domain.Hazard
	err := s.repository.WithinTx(ctx, func(store Store) error {
		hazard, err := store.GetHazard(ctx, hazardID)
		if err != nil {
			return err
		}
		if hazard.Version != expectedVersion {
			return domain.ErrConflict
		}
		queue, err := store.GetQueueEntry(ctx, hazardID)
		if err != nil {
			return err
		}

		hazardVersion, queueVersion := hazard.Version, queue.Version
		now := s.clock.Now()
		audit, err := change(&hazard, &queue, now)
		if err != nil {
			return err
		}
		hazard.Version++
		hazard.UpdatedAt = now
		queue.Version++
		queue.UpdatedAt = now
		if err := store.UpdateHazard(ctx, hazard, hazardVersion); err != nil {
			return err
		}
		if err := store.SaveQueueEntry(ctx, queue, queueVersion); err != nil {
			return err
		}
		if err := store.AppendAudit(ctx, domain.AuditEvent{
			ID: s.ids.New("audit"), HazardID: hazard.ID, SiteID: hazard.SiteID,
			EventType: eventType, ActorID: meta.ActorID, ActorRole: meta.Role,
			RuleID: audit.ruleID, RuleVersion: audit.ruleVersion, Reason: audit.reason,
			RequestID: meta.RequestID, SensitiveKeys: audit.sensitiveKeys,
			Details: audit.details, OccurredAt: now,
		}); err != nil {
			return err
		}
		changed = hazard
		return nil
	})
	if err != nil {
		return domain.Hazard{}, err
	}
	return changed, nil
}

func (s *HazardService) ManualDowngrade(ctx context.Context, hazardID, reason string, expectedVersion int64, meta RequestMeta) (domain.Hazard, error) {
	if !meta.Role.CanDowngrade() {
		return domain.Hazard{}, domain.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return domain.Hazard{}, domain.NewValidationError("reason", "人工降级理由至少十个字符")
	}
	changed, err := s.mutateQueueCase(
		ctx,
		hazardID,
		expectedVersion,
		"hazard.downgraded",
		meta,
		func(hazard *domain.Hazard, queue *domain.QueueEntry, now time.Time) (queueMutationAudit, error) {
			outcome, err := prepareDowngrade(hazard, queue, reason, now)
			if err != nil {
				return queueMutationAudit{}, err
			}
			outcome.apply(hazard, queue)
			return outcome.audit(), nil
		},
	)
	if err != nil {
		return domain.Hazard{}, fmt.Errorf("downgrade hazard: %w", err)
	}
	return changed, nil
}

type downgradeOutcome struct {
	reason       string
	nextNotifyAt time.Time
	ruleID       string
	ruleVersion  int64
	evidence     []string
}

func prepareDowngrade(
	hazard *domain.Hazard,
	queue *domain.QueueEntry,
	reason string,
	now time.Time,
) (downgradeOutcome, error) {
	if hazard == nil || queue == nil {
		return downgradeOutcome{}, domain.NewValidationError("hazard", "降级对象不能为空")
	}
	if !hazard.Escalated || hazard.Queue != domain.QueueFocus || queue.Queue != domain.QueueFocus {
		return downgradeOutcome{}, domain.NewValidationError("queue", "只有自动升级的重点隐患可以人工降级")
	}
	return downgradeOutcome{
		reason:       reason,
		nextNotifyAt: now.Add(24 * time.Hour),
	}, nil
}

func (d downgradeOutcome) apply(hazard *domain.Hazard, queue *domain.QueueEntry) {
	hazard.Queue = domain.QueueNormal
	hazard.Escalated = false
	hazard.EscalationReason = d.reason
	hazard.MatchedRuleID = ""
	hazard.MatchedRuleVersion = 0
	hazard.MatchedRuleEvidence = nil
	queue.Queue = domain.QueueNormal
	queue.EscalationTier = 0
	queue.NextNotifyAt = d.nextNotifyAt
}

func (d downgradeOutcome) audit() queueMutationAudit {
	return queueMutationAudit{
		reason:      d.reason,
		ruleID:      d.ruleID,
		ruleVersion: d.ruleVersion,
		details: map[string]any{
			"previous_evidence": append([]string(nil), d.evidence...),
			"to_queue":          domain.QueueNormal,
		},
	}
}

func (s *HazardService) AssignOwner(ctx context.Context, hazardID, ownerID string, expectedVersion int64, meta RequestMeta) (domain.Hazard, error) {
	if !meta.Role.CanConfigureRules() {
		return domain.Hazard{}, domain.ErrForbidden
	}
	ownerID = strings.TrimSpace(ownerID)
	if ownerID == "" {
		return domain.Hazard{}, domain.NewValidationError("owner_id", "处置负责人不能为空")
	}
	changed, err := s.mutateQueueCase(ctx, hazardID, expectedVersion, "hazard.assigned", meta, func(hazard *domain.Hazard, queue *domain.QueueEntry, _ time.Time) (queueMutationAudit, error) {
		if hazard.State == domain.StateClosed {
			return queueMutationAudit{}, domain.ErrInvalidTransition
		}
		hazard.OwnerID = ownerID
		queue.OwnerID = ownerID
		return queueMutationAudit{
			details: map[string]any{"owner_id": ownerID}, sensitiveKeys: []string{"owner_id"},
		}, nil
	})
	if err != nil {
		return domain.Hazard{}, fmt.Errorf("assign hazard: %w", err)
	}
	return changed, nil
}

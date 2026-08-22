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
	hazard, queue, err := s.loadQueueCase(ctx, hazardID)
	if err != nil {
		return domain.Hazard{}, err
	}
	// Optimistic concurrency: a stale client view must surface a conflict so the
	// caller can re-read and decide. Retrying by reloading the latest version and
	// re-applying the change would silently overwrite a concurrent winner — two
	// supervisors could both "succeed" assigning the same hazard while only the
	// last write survives. The transaction's version guard is the single source of
	// truth; a commit-time conflict is returned instead of resolved.
	if hazard.Version != expectedVersion {
		return domain.Hazard{}, domain.ErrConflict
	}
	changed, err := s.commitQueueMutation(ctx, hazard, queue, eventType, meta, change)
	if err != nil {
		return domain.Hazard{}, err
	}
	return changed, nil
}

func (s *HazardService) loadQueueCase(ctx context.Context, hazardID string) (domain.Hazard, domain.QueueEntry, error) {
	hazard, err := s.repository.GetHazard(ctx, hazardID)
	if err != nil {
		return domain.Hazard{}, domain.QueueEntry{}, err
	}
	queue, err := s.repository.GetQueueEntry(ctx, hazardID)
	if err != nil {
		return domain.Hazard{}, domain.QueueEntry{}, err
	}
	return hazard, queue, nil
}

func (s *HazardService) commitQueueMutation(
	ctx context.Context,
	hazard domain.Hazard,
	queue domain.QueueEntry,
	eventType string,
	meta RequestMeta,
	change queueMutation,
) (domain.Hazard, error) {
	hazardVersion, queueVersion := hazard.Version, queue.Version
	now := s.clock.Now()
	audit, err := change(&hazard, &queue, now)
	if err != nil {
		return domain.Hazard{}, err
	}
	hazard.Version++
	hazard.UpdatedAt = now
	queue.Version++
	queue.UpdatedAt = now
	err = s.repository.WithinTx(ctx, func(store Store) error {
		if err := store.UpdateHazard(ctx, hazard, hazardVersion); err != nil {
			return err
		}
		if err := store.SaveQueueEntry(ctx, queue, queueVersion); err != nil {
			return err
		}
		return store.AppendAudit(ctx, domain.AuditEvent{
			ID: s.ids.New("audit"), HazardID: hazard.ID, SiteID: hazard.SiteID,
			EventType: eventType, ActorID: meta.ActorID, ActorRole: meta.Role,
			RuleID: audit.ruleID, RuleVersion: audit.ruleVersion, Reason: audit.reason,
			RequestID: meta.RequestID, SensitiveKeys: audit.sensitiveKeys,
			Details: audit.details, OccurredAt: now,
		})
	})
	if err != nil {
		return domain.Hazard{}, err
	}
	return hazard, nil
}

func (s *HazardService) ManualDowngrade(ctx context.Context, hazardID, reason string, expectedVersion int64, meta RequestMeta) (domain.Hazard, error) {
	if !meta.Role.CanDowngrade() {
		return domain.Hazard{}, domain.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if len(reason) < 10 {
		return domain.Hazard{}, domain.NewValidationError("reason", "人工降级理由至少十个字符")
	}
	changed, err := s.mutateQueueCase(ctx, hazardID, expectedVersion, "hazard.downgraded", meta, func(hazard *domain.Hazard, queue *domain.QueueEntry, now time.Time) (queueMutationAudit, error) {
		if !hazard.Escalated || hazard.Queue != domain.QueueFocus {
			return queueMutationAudit{}, domain.NewValidationError("queue", "只有自动升级的重点隐患可以人工降级")
		}
		audit := queueMutationAudit{
			reason: reason, ruleID: hazard.MatchedRuleID, ruleVersion: hazard.MatchedRuleVersion,
			details: map[string]any{
				"previous_evidence": append([]string(nil), hazard.MatchedRuleEvidence...),
				"to_queue":          domain.QueueNormal,
			},
		}
		hazard.Queue = domain.QueueNormal
		hazard.Escalated = false
		hazard.EscalationReason = reason
		queue.Queue = domain.QueueNormal
		queue.EscalationTier = 0
		queue.NextNotifyAt = now.Add(24 * time.Hour)
		return audit, nil
	})
	if err != nil {
		return domain.Hazard{}, fmt.Errorf("downgrade hazard: %w", err)
	}
	return changed, nil
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

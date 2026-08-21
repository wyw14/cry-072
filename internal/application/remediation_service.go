package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
)

type RemediationService struct {
	repository Repository
	clock      platform.TimeSource
	ids        platform.IDGenerator
}

func NewRemediationService(repository Repository, clock platform.TimeSource, ids platform.IDGenerator) *RemediationService {
	return &RemediationService{repository: repository, clock: clock, ids: ids}
}

func (s *RemediationService) SavePlan(ctx context.Context, plan domain.RemediationPlan, expectedVersion int64, meta RequestMeta) (domain.RemediationPlan, error) {
	if err := plan.Validate(); err != nil {
		return domain.RemediationPlan{}, err
	}
	hazard, err := s.repository.GetHazard(ctx, plan.HazardID)
	if err != nil {
		return domain.RemediationPlan{}, fmt.Errorf("load hazard: %w", err)
	}
	if hazard.State == domain.StateClosed {
		return domain.RemediationPlan{}, domain.ErrInvalidTransition
	}
	now := s.clock.Now()
	if plan.ID == "" {
		plan.ID = s.ids.New("plan")
		plan.Version = 1
		plan.CreatedAt = now
		plan.CreatedBy = meta.ActorID
	} else {
		plan.Version = expectedVersion + 1
	}
	for index := range plan.Tasks {
		if plan.Tasks[index].ID == "" {
			plan.Tasks[index].ID = s.ids.New("task")
		}
	}
	plan.UpdatedAt = now
	err = s.repository.WithinTx(ctx, func(store Store) error {
		if err := store.SaveRemediationPlan(ctx, plan, expectedVersion); err != nil {
			return err
		}
		if plan.TemporaryBlock {
			site, err := store.GetSite(ctx, hazard.SiteID)
			if err != nil {
				return err
			}
			if site.State != domain.SiteIsolated {
				oldVersion := site.Version
				site.State = domain.SiteIsolated
				site.Version++
				site.UpdatedAt = now
				if err := store.UpdateSite(ctx, site, oldVersion); err != nil {
					return err
				}
			}
		}
		return store.AppendAudit(ctx, domain.AuditEvent{
			ID: s.ids.New("audit"), HazardID: hazard.ID, SiteID: hazard.SiteID,
			EventType: "remediation.plan_saved", ActorID: meta.ActorID, ActorRole: meta.Role,
			RequestID: meta.RequestID, OccurredAt: now,
			Details: map[string]any{"temporary_block": plan.TemporaryBlock, "task_count": len(plan.Tasks)},
		})
	})
	if err != nil {
		return domain.RemediationPlan{}, fmt.Errorf("save remediation plan: %w", err)
	}
	return plan, nil
}

func (s *RemediationService) CompleteTask(ctx context.Context, hazardID, taskID string, evidenceIDs []string, expectedVersion int64, meta RequestMeta) (domain.RemediationPlan, error) {
	if len(evidenceIDs) == 0 {
		return domain.RemediationPlan{}, domain.NewValidationError("evidence_ids", "完成整改任务必须提交证据")
	}
	plan, err := s.repository.GetRemediationPlan(ctx, hazardID)
	if err != nil {
		return domain.RemediationPlan{}, err
	}
	if plan.Version != expectedVersion {
		return domain.RemediationPlan{}, domain.ErrConflict
	}
	found := false
	for index := range plan.Tasks {
		if plan.Tasks[index].ID != taskID {
			continue
		}
		if !plan.Tasks[index].CompletedAt.IsZero() {
			return plan, nil
		}
		plan.Tasks[index].CompletedAt = s.clock.Now()
		plan.Tasks[index].EvidenceIDs = append([]string(nil), evidenceIDs...)
		found = true
		break
	}
	if !found {
		return domain.RemediationPlan{}, domain.ErrNotFound
	}
	plan.Version++
	plan.UpdatedAt = s.clock.Now()
	if err := s.repository.SaveRemediationPlan(ctx, plan, expectedVersion); err != nil {
		return domain.RemediationPlan{}, fmt.Errorf("complete remediation task: %w", err)
	}
	return plan, nil
}

func (s *RemediationService) Reinspect(ctx context.Context, input domain.Reinspection, expectedHazardVersion int64, meta RequestMeta) (domain.Hazard, error) {
	if strings.TrimSpace(input.IdempotencyKey) == "" {
		return domain.Hazard{}, domain.NewValidationError("idempotency_key", "复检幂等键不能为空")
	}
	input.InspectorID = meta.ActorID
	if err := input.Validate(); err != nil {
		return domain.Hazard{}, err
	}
	hazard, err := s.repository.GetHazard(ctx, input.HazardID)
	if err != nil {
		return domain.Hazard{}, err
	}
	if hazard.Version != expectedHazardVersion {
		return domain.Hazard{}, domain.ErrConflict
	}
	if hazard.State != domain.StateReview {
		return domain.Hazard{}, domain.NewValidationError("state", "只有待复核隐患可以登记复检")
	}
	if !meta.Role.CanReview() {
		return domain.Hazard{}, domain.ErrForbidden
	}
	input.ID = s.ids.New("reinspection")
	input.CompletedAt = s.clock.Now()
	previousState := hazard.State
	if input.Passed {
		hazard.State = domain.StateResolved
		timestamp := input.CompletedAt
		hazard.PassedReinspectionAt = &timestamp
		hazard.LastReinspectionID = input.ID
	} else {
		hazard.State = domain.StateHandling
		hazard.PassedReinspectionAt = nil
		hazard.LastReinspectionID = input.ID
	}
	hazard.Version++
	hazard.UpdatedAt = input.CompletedAt
	err = s.repository.WithinTx(ctx, func(store Store) error {
		if err := store.SaveReinspection(ctx, input); err != nil {
			return err
		}
		if err := store.UpdateHazard(ctx, hazard, expectedHazardVersion); err != nil {
			return err
		}
		if input.Passed && input.RestoreSite {
			openHazards, err := store.ListOpenHazardsForSite(ctx, hazard.SiteID)
			if err != nil {
				return err
			}
			blocking := false
			for _, open := range openHazards {
				if open.ID != hazard.ID && open.State != domain.StateClosed && open.IsFocus() {
					blocking = true
					break
				}
			}
			if !blocking {
				site, err := store.GetSite(ctx, hazard.SiteID)
				if err != nil {
					return err
				}
				if site.State == domain.SiteIsolated {
					oldVersion := site.Version
					site.State = domain.SiteActive
					site.Version++
					site.UpdatedAt = input.CompletedAt
					if err := store.UpdateSite(ctx, site, oldVersion); err != nil {
						return err
					}
				}
			}
		}
		return store.AppendAudit(ctx, domain.AuditEvent{
			ID: s.ids.New("audit"), HazardID: hazard.ID, SiteID: hazard.SiteID,
			EventType: "hazard.reinspected", ActorID: meta.ActorID, ActorRole: meta.Role,
			PreviousState: previousState, CurrentState: hazard.State, Reason: input.Notes,
			RequestID: meta.RequestID, OccurredAt: input.CompletedAt,
			Details: map[string]any{"passed": input.Passed, "observed_score": input.ObservedScore, "restore_requested": input.RestoreSite},
		})
	})
	if err != nil {
		return domain.Hazard{}, fmt.Errorf("reinspect hazard: %w", err)
	}
	return hazard, nil
}

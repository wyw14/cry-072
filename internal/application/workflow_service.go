package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
)

type WorkflowService struct {
	repository Repository
	clock      platform.TimeSource
	ids        platform.IDGenerator
}

func NewWorkflowService(repository Repository, clock platform.TimeSource, ids platform.IDGenerator) *WorkflowService {
	return &WorkflowService{repository: repository, clock: clock, ids: ids}
}

func (s *WorkflowService) Transition(ctx context.Context, hazardID string, expectedVersion int64, request domain.TransitionRequest, meta RequestMeta) (domain.Hazard, error) {
	hazard, err := s.repository.GetHazard(ctx, hazardID)
	if err != nil {
		return domain.Hazard{}, fmt.Errorf("load hazard: %w", err)
	}
	if hazard.Version != expectedVersion {
		return domain.Hazard{}, domain.ErrConflict
	}
	request.ActorID = meta.ActorID
	request.ActorRole = meta.Role
	if err := domain.ValidateTransition(hazard, request); err != nil {
		return domain.Hazard{}, err
	}
	if request.TargetState == domain.StateReview {
		plan, err := s.repository.GetRemediationPlan(ctx, hazard.ID)
		if err != nil {
			return domain.Hazard{}, fmt.Errorf("load remediation plan: %w", err)
		}
		if !plan.Complete() {
			return domain.Hazard{}, domain.NewValidationError("remediation_plan", "整改任务全部完成后才能申请复核")
		}
	}
	if request.TargetState == domain.StateHandling && strings.TrimSpace(request.Reason) == "" {
		return domain.Hazard{}, domain.NewValidationError("reason", "开始处理或退回处理必须填写原因")
	}
	previous := hazard.State
	hazard.State = request.TargetState
	hazard.Version++
	hazard.UpdatedAt = s.clock.Now()
	err = commitAudited(ctx, s.repository, auditedChange{
		operation: "transition hazard",
		apply: func(store Store) error {
			return store.UpdateHazard(ctx, hazard, expectedVersion)
		},
		event: domain.AuditEvent{
			ID: s.ids.New("audit"), HazardID: hazard.ID, SiteID: hazard.SiteID,
			EventType: "hazard.state_changed", ActorID: meta.ActorID, ActorRole: meta.Role,
			PreviousState: previous, CurrentState: hazard.State, Reason: request.Reason,
			RequestID: meta.RequestID, OccurredAt: hazard.UpdatedAt,
		},
	})
	if err != nil {
		return domain.Hazard{}, err
	}
	return hazard, nil
}

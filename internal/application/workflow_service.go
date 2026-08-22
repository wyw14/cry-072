package application

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	transition := newHazardTransition(hazard, expectedVersion, request, meta, s.clock.Now())
	if err := transition.validate(); err != nil {
		return domain.Hazard{}, err
	}
	if err := transition.validateBusinessPrerequisites(ctx, s.repository); err != nil {
		return domain.Hazard{}, err
	}
	changed := transition.apply()
	err = commitAudited(ctx, s.repository, auditedChange{
		operation: "transition hazard",
		apply: func(store Store) error {
			return store.UpdateHazard(ctx, changed, expectedVersion)
		},
		event: transition.auditEvent(s.ids.New("audit"), changed),
	})
	if err != nil {
		return domain.Hazard{}, err
	}
	return changed, nil
}

type hazardTransition struct {
	current         domain.Hazard
	expectedVersion int64
	request         domain.TransitionRequest
	meta            RequestMeta
	changedAt       time.Time
}

func newHazardTransition(
	current domain.Hazard,
	expectedVersion int64,
	request domain.TransitionRequest,
	meta RequestMeta,
	changedAt time.Time,
) hazardTransition {
	request.ActorID = meta.ActorID
	request.ActorRole = meta.Role
	return hazardTransition{
		current: current, expectedVersion: expectedVersion, request: request,
		meta: meta, changedAt: changedAt,
	}
}

func (t hazardTransition) validate() error {
	if t.current.Version != t.expectedVersion {
		return domain.ErrConflict
	}
	return domain.ValidateTransition(t.current, t.request)
}

func (t hazardTransition) validateBusinessPrerequisites(_ context.Context, _ Repository) error {
	if t.request.TargetState == domain.StateHandling && strings.TrimSpace(t.request.Reason) == "" {
		return domain.NewValidationError("reason", "开始处理或退回处理必须填写原因")
	}
	return nil
}

func (t hazardTransition) apply() domain.Hazard {
	changed := t.current
	changed.State = t.request.TargetState
	changed.Version++
	changed.UpdatedAt = t.changedAt
	return changed
}

func (t hazardTransition) auditEvent(id string, changed domain.Hazard) domain.AuditEvent {
	return domain.AuditEvent{
		ID: id, HazardID: changed.ID, SiteID: changed.SiteID,
		EventType: "hazard.state_changed", ActorID: t.meta.ActorID, ActorRole: t.meta.Role,
		PreviousState: t.current.State, CurrentState: changed.State, Reason: t.request.Reason,
		RequestID: t.meta.RequestID, OccurredAt: changed.UpdatedAt,
	}
}

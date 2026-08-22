package domain

import "fmt"

var allowedTransitions = map[HazardState]map[HazardState]struct{}{
	StatePending: {
		StateHandling: {},
	},
	StateHandling: {
		StateReview: {},
	},
	StateReview: {
		StateHandling: {},
		StateResolved: {},
	},
	StateResolved: {
		StateHandling: {},
		StateClosed:   {},
	},
}

type TransitionRequest struct {
	TargetState HazardState `json:"target_state"`
	Reason      string      `json:"reason"`
	ActorID     string      `json:"actor_id"`
	ActorRole   Role        `json:"actor_role"`
}

func ValidateTransition(h Hazard, request TransitionRequest) error {
	policy := transitionPolicy{hazard: h, request: request}
	checks := []func() error{
		policy.validateTarget,
		policy.validateEdge,
		policy.validateActor,
		policy.validateReviewAuthority,
		policy.validateClosure,
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

type transitionPolicy struct {
	hazard  Hazard
	request TransitionRequest
}

func (p transitionPolicy) validateTarget() error {
	if !p.request.TargetState.Valid() {
		return NewValidationError("target_state", "目标状态无效")
	}
	if p.hazard.State == p.request.TargetState {
		return fmt.Errorf("%w: state unchanged", ErrInvalidTransition)
	}
	return nil
}

func (p transitionPolicy) validateEdge() error {
	next, exists := allowedTransitions[p.hazard.State]
	if !exists {
		return fmt.Errorf("%w: no transitions from %s", ErrInvalidTransition, p.hazard.State)
	}
	if _, allowed := next[p.request.TargetState]; !allowed {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, p.hazard.State, p.request.TargetState)
	}
	return nil
}

func (p transitionPolicy) validateActor() error {
	if p.request.ActorID == "" {
		return NewValidationError("actor_id", "操作人不能为空")
	}
	return nil
}

func (p transitionPolicy) validateReviewAuthority() error {
	if p.request.TargetState != StateResolved && p.request.TargetState != StateClosed {
		return nil
	}
	if !p.request.ActorRole.CanReview() {
		return ErrForbidden
	}
	return nil
}

func (p transitionPolicy) validateClosure() error {
	if p.request.TargetState != StateClosed || !p.hazard.IsFocus() {
		return nil
	}
	if p.hazard.PassedReinspectionAt == nil {
		return NewValidationError("reinspection", "重点隐患必须通过复检后才能关闭")
	}
	return nil
}

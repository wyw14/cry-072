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
	if !request.TargetState.Valid() {
		return NewValidationError("target_state", "目标状态无效")
	}
	if h.State == request.TargetState {
		return fmt.Errorf("%w: state unchanged", ErrInvalidTransition)
	}
	if _, ok := allowedTransitions[h.State][request.TargetState]; !ok {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, h.State, request.TargetState)
	}
	if request.ActorID == "" {
		return NewValidationError("actor_id", "操作人不能为空")
	}
	if request.TargetState == StateResolved && !request.ActorRole.CanReview() {
		return ErrForbidden
	}
	if request.TargetState == StateClosed {
		if !request.ActorRole.CanReview() {
			return ErrForbidden
		}
		if err := h.CanClose(); err != nil {
			return err
		}
	}
	return nil
}

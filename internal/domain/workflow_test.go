package domain

import (
	"errors"
	"testing"
	"time"
)

func TestFocusHazardCannotCloseWithoutPassedReinspection(t *testing.T) {
	hazard := Hazard{State: StateResolved, Queue: QueueFocus, Escalated: true}
	err := ValidateTransition(hazard, TransitionRequest{TargetState: StateClosed, ActorID: "reviewer", ActorRole: RoleReviewer})
	if err == nil {
		t.Fatal("expected reinspection gate")
	}
	passed := time.Now()
	hazard.PassedReinspectionAt = &passed
	if err := ValidateTransition(hazard, TransitionRequest{TargetState: StateClosed, ActorID: "reviewer", ActorRole: RoleReviewer}); err != nil {
		t.Fatalf("expected close to be allowed: %v", err)
	}
}

func TestFocusHazardCannotCloseWithOnlyFailedReinspection(t *testing.T) {
	// 重点隐患存在一条未通过的复检记录，状态被人工推进到已解除后仍不应允许关闭。
	hazard := Hazard{
		State:               StateResolved,
		Queue:               QueueFocus,
		Escalated:           true,
		LastReinspectionID:  "reinspection_failed",
		PassedReinspectionAt: nil,
	}
	err := ValidateTransition(hazard, TransitionRequest{TargetState: StateClosed, ActorID: "reviewer", ActorRole: RoleReviewer})
	if err == nil {
		t.Fatal("expected failed reinspection to block closure")
	}
	// 通过复检后方可关闭。
	passed := time.Now()
	hazard.PassedReinspectionAt = &passed
	if err := ValidateTransition(hazard, TransitionRequest{TargetState: StateClosed, ActorID: "reviewer", ActorRole: RoleReviewer}); err != nil {
		t.Fatalf("expected close to be allowed after passed reinspection: %v", err)
	}
}

func TestReviewerRoleRequiredForResolution(t *testing.T) {
	hazard := Hazard{State: StateReview, Queue: QueueNormal}
	err := ValidateTransition(hazard, TransitionRequest{TargetState: StateResolved, ActorID: "worker", ActorRole: RoleInspector})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestAuditPublicDetailsRedactsSensitiveKeys(t *testing.T) {
	event := AuditEvent{Details: map[string]any{"owner": "u1", "queue": "focus"}, SensitiveKeys: []string{"owner"}}
	public := event.PublicDetails()
	if public["owner"] != "***" || public["queue"] != "focus" {
		t.Fatalf("unexpected public details: %#v", public)
	}
}

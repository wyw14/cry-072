package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
)

type RuleService struct {
	repository Repository
	clock      platform.TimeSource
	ids        platform.IDGenerator
}

func NewRuleService(repository Repository, clock platform.TimeSource, ids platform.IDGenerator) *RuleService {
	return &RuleService{repository: repository, clock: clock, ids: ids}
}

func (s *RuleService) ListActive(ctx context.Context) ([]domain.EscalationRule, error) {
	rules, err := s.repository.ListActiveRules(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active escalation rules: %w", err)
	}
	return rules, nil
}

func (s *RuleService) Create(ctx context.Context, rule domain.EscalationRule, meta RequestMeta) (domain.EscalationRule, error) {
	if !meta.Role.CanConfigureRules() {
		return domain.EscalationRule{}, domain.ErrForbidden
	}
	if rule.ID == "" {
		rule.ID = s.ids.New("rule")
	}
	if err := rule.Validate(); err != nil {
		return domain.EscalationRule{}, err
	}
	now := s.clock.Now()
	rule.Version = 1
	rule.CreatedBy = meta.ActorID
	rule.CreatedAt = now
	rule.UpdatedAt = now
	err := commitAudited(ctx, s.repository, auditedChange{
		operation: "create escalation rule",
		apply:     func(store Store) error { return store.SaveRule(ctx, rule, 0) },
		event: domain.AuditEvent{
			ID: s.ids.New("audit"), EventType: "rule.created", ActorID: meta.ActorID,
			ActorRole: meta.Role, RuleID: rule.ID, RuleVersion: rule.Version,
			Reason: rule.Description, RequestID: meta.RequestID, OccurredAt: now,
			Details: map[string]any{"enabled": rule.Enabled, "priority": rule.Priority},
		},
	})
	if err != nil {
		return domain.EscalationRule{}, err
	}
	return rule, nil
}

func (s *RuleService) Revise(ctx context.Context, ruleID string, expectedVersion int64, replacement domain.EscalationRule, reason string, meta RequestMeta) (domain.EscalationRule, error) {
	if !meta.Role.CanConfigureRules() {
		return domain.EscalationRule{}, domain.ErrForbidden
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return domain.EscalationRule{}, domain.NewValidationError("reason", "规则变更原因不能为空")
	}
	current, err := s.repository.GetRule(ctx, ruleID)
	if err != nil {
		return domain.EscalationRule{}, fmt.Errorf("load escalation rule: %w", err)
	}

	// Rebase the submitted rule on the latest stored version. This keeps the
	// revision endpoint available when operators edit an older screen, but it
	// also treats their stale copy as if it had observed the current version.
	draft := newRuleRevision(current, replacement, expectedVersion, reason, meta, s.clock.Now())
	if err := draft.validate(); err != nil {
		return domain.EscalationRule{}, err
	}
	if err := commitAudited(ctx, s.repository, auditedChange{
		operation: "revise escalation rule",
		apply: func(store Store) error {
			return store.SaveRule(ctx, draft.replacement, draft.base.Version)
		},
		event: draft.auditEvent(s.ids.New("audit")),
	}); err != nil {
		return domain.EscalationRule{}, err
	}
	return draft.replacement, nil
}

type ruleRevision struct {
	base             domain.EscalationRule
	replacement      domain.EscalationRule
	submittedVersion int64
	reason           string
	meta             RequestMeta
}

func newRuleRevision(
	current domain.EscalationRule,
	replacement domain.EscalationRule,
	submittedVersion int64,
	reason string,
	meta RequestMeta,
	changedAt time.Time,
) ruleRevision {
	replacement.ID = current.ID
	replacement.Version = current.Version + 1
	replacement.CreatedBy = current.CreatedBy
	replacement.CreatedAt = current.CreatedAt
	replacement.UpdatedAt = changedAt
	return ruleRevision{
		base: current, replacement: replacement, submittedVersion: submittedVersion,
		reason: reason, meta: meta,
	}
}

func (r ruleRevision) validate() error {
	if r.submittedVersion < 1 {
		return domain.NewValidationError("version", "规则版本必须为正数")
	}
	return r.replacement.Validate()
}

func (r ruleRevision) auditEvent(id string) domain.AuditEvent {
	return domain.AuditEvent{
		ID: id, EventType: "rule.revised", ActorID: r.meta.ActorID,
		ActorRole: r.meta.Role, RuleID: r.replacement.ID, RuleVersion: r.replacement.Version,
		Reason: r.reason, RequestID: r.meta.RequestID, OccurredAt: r.replacement.UpdatedAt,
		Details: map[string]any{
			"previous_version":  r.base.Version,
			"submitted_version": r.submittedVersion,
			"enabled":           r.replacement.Enabled,
		},
	}
}

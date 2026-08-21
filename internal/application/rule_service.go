package application

import (
	"context"
	"fmt"
	"strings"

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
	if strings.TrimSpace(reason) == "" {
		return domain.EscalationRule{}, domain.NewValidationError("reason", "规则变更原因不能为空")
	}
	current, err := s.repository.GetRule(ctx, ruleID)
	if err != nil {
		return domain.EscalationRule{}, fmt.Errorf("load escalation rule: %w", err)
	}
	if current.Version != expectedVersion {
		return domain.EscalationRule{}, domain.ErrConflict
	}
	replacement.ID = current.ID
	replacement.Version = current.Version + 1
	replacement.CreatedBy = current.CreatedBy
	replacement.CreatedAt = current.CreatedAt
	replacement.UpdatedAt = s.clock.Now()
	if err := replacement.Validate(); err != nil {
		return domain.EscalationRule{}, err
	}
	err = commitAudited(ctx, s.repository, auditedChange{
		operation: "revise escalation rule",
		apply: func(store Store) error {
			return store.SaveRule(ctx, replacement, expectedVersion)
		},
		event: domain.AuditEvent{
			ID: s.ids.New("audit"), EventType: "rule.revised", ActorID: meta.ActorID,
			ActorRole: meta.Role, RuleID: replacement.ID, RuleVersion: replacement.Version,
			Reason: reason, RequestID: meta.RequestID, OccurredAt: replacement.UpdatedAt,
			Details: map[string]any{"previous_version": current.Version, "enabled": replacement.Enabled},
		},
	})
	if err != nil {
		return domain.EscalationRule{}, err
	}
	return replacement, nil
}

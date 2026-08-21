package service

import (
	"time"

	"github.com/wyw14/cry-072/internal/domain"
)

type DeadlinePolicy struct {
	NormalFallback time.Duration
	FocusFallback  time.Duration
}

func NewDeadlinePolicy() DeadlinePolicy {
	return DeadlinePolicy{NormalFallback: 72 * time.Hour, FocusFallback: 8 * time.Hour}
}

func (p DeadlinePolicy) ForHazard(hazard domain.Hazard, category domain.RiskCategory, matchedRule *domain.EscalationRule, now time.Time) time.Time {
	if hazard.Queue == domain.QueueFocus {
		if matchedRule != nil && matchedRule.FocusSLA > 0 {
			return now.Add(matchedRule.FocusSLA)
		}
		return now.Add(p.FocusFallback)
	}
	if category.DefaultSLA > 0 {
		return now.Add(time.Duration(category.DefaultSLA) * time.Minute)
	}
	return now.Add(p.NormalFallback)
}

func (p DeadlinePolicy) NextNotification(queue domain.QueueEntry, rule *domain.EscalationRule, now time.Time) time.Time {
	if rule != nil {
		if queue.LastNotifiedAt.IsZero() && rule.NotifyAfter >= 0 {
			return now.Add(rule.NotifyAfter)
		}
		if !queue.LastNotifiedAt.IsZero() && rule.ReNotifyEvery > 0 {
			return queue.LastNotifiedAt.Add(rule.ReNotifyEvery)
		}
	}
	if queue.Queue == domain.QueueFocus {
		return now.Add(time.Hour)
	}
	return now.Add(24 * time.Hour)
}

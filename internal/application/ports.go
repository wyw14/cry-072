package application

import (
	"context"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
)

type Store interface {
	CreateSite(context.Context, domain.Site) error
	UpdateSite(context.Context, domain.Site, int64) error
	GetSite(context.Context, string) (domain.Site, error)
	CreateFacility(context.Context, domain.Facility) error
	GetFacility(context.Context, string) (domain.Facility, error)
	CreateRiskCategory(context.Context, domain.RiskCategory) error
	GetRiskCategory(context.Context, string) (domain.RiskCategory, error)
	SaveRule(context.Context, domain.EscalationRule, int64) error
	GetRule(context.Context, string) (domain.EscalationRule, error)
	ListActiveRules(context.Context) ([]domain.EscalationRule, error)
	SaveInspection(context.Context, domain.Inspection) error
	SaveHazard(context.Context, domain.Hazard) error
	UpdateHazard(context.Context, domain.Hazard, int64) error
	GetHazard(context.Context, string) (domain.Hazard, error)
	FindHazardByIdempotency(context.Context, string) (domain.Hazard, error)
	SaveEvidence(context.Context, domain.Evidence) error
	ListEvidence(context.Context, string) ([]domain.Evidence, error)
	SaveQueueEntry(context.Context, domain.QueueEntry, int64) error
	GetQueueEntry(context.Context, string) (domain.QueueEntry, error)
	ListQueue(context.Context, domain.QueueFilter, time.Time) (domain.Page[domain.Hazard], error)
	ListDueQueueEntries(context.Context, time.Time, int) ([]domain.QueueEntry, error)
	SaveRemediationPlan(context.Context, domain.RemediationPlan, int64) error
	GetRemediationPlan(context.Context, string) (domain.RemediationPlan, error)
	SaveReinspection(context.Context, domain.Reinspection) error
	ListOpenHazardsForSite(context.Context, string) ([]domain.Hazard, error)
	SaveNotification(context.Context, domain.Notification) error
	FindNotificationByDeduplication(context.Context, string) (domain.Notification, error)
	SaveNotificationAttempt(context.Context, domain.NotificationAttempt) error
	AppendAudit(context.Context, domain.AuditEvent) error
	ListAudit(context.Context, string, int, int) (domain.Page[domain.AuditEvent], error)
	ListHazardsBetween(context.Context, time.Time, time.Time) ([]domain.Hazard, error)
	CheckReady(context.Context) error
}

type Repository interface {
	Store
	WithinTx(context.Context, func(Store) error) error
}

type RequestMeta struct {
	ActorID   string
	Role      domain.Role
	RequestID string
}

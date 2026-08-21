package memory

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/wyw14/cry-072/internal/application"
	"github.com/wyw14/cry-072/internal/domain"
)

type catalogLedger struct {
	sites      map[string]domain.Site
	facilities map[string]domain.Facility
	categories map[string]domain.RiskCategory
	rules      map[string][]domain.EscalationRule
}

type hazardLedger struct {
	inspections   map[string]domain.Inspection
	hazards       map[string]domain.Hazard
	idempotency   map[string]string
	evidence      map[string][]domain.Evidence
	queues        map[string]domain.QueueEntry
	plans         map[string]domain.RemediationPlan
	reinspections map[string][]domain.Reinspection
}

type deliveryLedger struct {
	notifications        map[string]domain.Notification
	notificationByDedup  map[string]string
	notificationAttempts []domain.NotificationAttempt
}

type auditLedger struct {
	events []domain.AuditEvent
}

type state struct {
	catalog  catalogLedger
	hazards  hazardLedger
	delivery deliveryLedger
	audit    auditLedger
}

func newState() *state {
	return &state{
		catalog: catalogLedger{
			sites: make(map[string]domain.Site), facilities: make(map[string]domain.Facility),
			categories: make(map[string]domain.RiskCategory), rules: make(map[string][]domain.EscalationRule),
		},
		hazards: hazardLedger{
			inspections: make(map[string]domain.Inspection), hazards: make(map[string]domain.Hazard),
			idempotency: make(map[string]string), evidence: make(map[string][]domain.Evidence),
			queues: make(map[string]domain.QueueEntry), plans: make(map[string]domain.RemediationPlan),
			reinspections: make(map[string][]domain.Reinspection),
		},
		delivery: deliveryLedger{
			notifications:        make(map[string]domain.Notification),
			notificationByDedup:  make(map[string]string),
			notificationAttempts: make([]domain.NotificationAttempt, 0),
		},
		audit: auditLedger{events: make([]domain.AuditEvent, 0)},
	}
}

func (s *state) clone() *state {
	copyState := &state{
		catalog: catalogLedger{
			sites: maps.Clone(s.catalog.sites), facilities: maps.Clone(s.catalog.facilities),
			categories: maps.Clone(s.catalog.categories),
			rules:      make(map[string][]domain.EscalationRule, len(s.catalog.rules)),
		},
		hazards: hazardLedger{
			inspections:   make(map[string]domain.Inspection, len(s.hazards.inspections)),
			hazards:       make(map[string]domain.Hazard, len(s.hazards.hazards)),
			idempotency:   maps.Clone(s.hazards.idempotency),
			evidence:      make(map[string][]domain.Evidence, len(s.hazards.evidence)),
			queues:        maps.Clone(s.hazards.queues),
			plans:         make(map[string]domain.RemediationPlan, len(s.hazards.plans)),
			reinspections: make(map[string][]domain.Reinspection, len(s.hazards.reinspections)),
		},
		delivery: deliveryLedger{
			notifications:        maps.Clone(s.delivery.notifications),
			notificationByDedup:  maps.Clone(s.delivery.notificationByDedup),
			notificationAttempts: slices.Clone(s.delivery.notificationAttempts),
		},
		audit: auditLedger{events: make([]domain.AuditEvent, len(s.audit.events))},
	}
	for key, history := range s.catalog.rules {
		copyState.catalog.rules[key] = slices.Clone(history)
	}
	for key, inspection := range s.hazards.inspections {
		copyState.hazards.inspections[key] = cloneInspection(inspection)
	}
	for key, hazard := range s.hazards.hazards {
		copyState.hazards.hazards[key] = cloneHazard(hazard)
	}
	for key, items := range s.hazards.evidence {
		copyState.hazards.evidence[key] = slices.Clone(items)
	}
	for key, plan := range s.hazards.plans {
		copyState.hazards.plans[key] = clonePlan(plan)
	}
	for key, history := range s.hazards.reinspections {
		copyState.hazards.reinspections[key] = slices.Clone(history)
	}
	for index, event := range s.audit.events {
		copyState.audit.events[index] = cloneAudit(event)
	}
	return copyState
}

type Repository struct {
	mu    sync.RWMutex
	state *state
	*store
}

func New() *Repository {
	repository := &Repository{state: newState()}
	repository.store = &store{owner: repository}
	return repository
}

func (r *Repository) WithinTx(ctx context.Context, callback func(application.Store) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	working := r.state.clone()
	if err := callback(&store{snapshot: working}); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	r.state = working
	return nil
}

type store struct {
	owner    *Repository
	snapshot *state
}

func (s *store) withRead(ctx context.Context, operation func(*state) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.snapshot != nil {
		return operation(s.snapshot)
	}
	if s.owner == nil {
		return errors.New("memory repository is not initialized")
	}
	s.owner.mu.RLock()
	defer s.owner.mu.RUnlock()
	if s.owner.state == nil {
		return errors.New("memory repository is not initialized")
	}
	return operation(s.owner.state)
}

func (s *store) withWrite(ctx context.Context, operation func(*state) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if s.snapshot != nil {
		return operation(s.snapshot)
	}
	if s.owner == nil {
		return errors.New("memory repository is not initialized")
	}
	s.owner.mu.Lock()
	defer s.owner.mu.Unlock()
	if s.owner.state == nil {
		return errors.New("memory repository is not initialized")
	}
	return operation(s.owner.state)
}

func readFrom[T any](ctx context.Context, s *store, operation func(*state) (T, error)) (T, error) {
	var result T
	err := s.withRead(ctx, func(image *state) error {
		value, err := operation(image)
		if err != nil {
			return err
		}
		result = value
		return nil
	})
	return result, err
}

func findByID[V any](values map[string]V, id string) (V, error) {
	value, exists := values[id]
	if !exists {
		var zero V
		return zero, domain.ErrNotFound
	}
	return value, nil
}

func saveVersioned[V any](values map[string]V, id string, value V, expected int64, versionOf func(V) int64) error {
	current, exists := values[id]
	switch {
	case expected == 0 && exists:
		return domain.ErrDuplicate
	case expected > 0 && !exists:
		return domain.ErrNotFound
	case expected > 0 && versionOf(current) != expected:
		return domain.ErrConflict
	default:
		values[id] = value
		return nil
	}
}

func (s *store) CreateSite(ctx context.Context, value domain.Site) error {
	return s.withWrite(ctx, func(image *state) error {
		if _, exists := image.catalog.sites[value.ID]; exists {
			return domain.ErrDuplicate
		}
		for _, current := range image.catalog.sites {
			if current.Code == value.Code {
				return domain.ErrDuplicate
			}
		}
		image.catalog.sites[value.ID] = value
		return nil
	})
}

func (s *store) UpdateSite(ctx context.Context, value domain.Site, expected int64) error {
	return s.withWrite(ctx, func(image *state) error {
		current, ok := image.catalog.sites[value.ID]
		if !ok {
			return domain.ErrNotFound
		}
		if current.Version != expected {
			return domain.ErrConflict
		}
		image.catalog.sites[value.ID] = value
		return nil
	})
}

func (s *store) GetSite(ctx context.Context, id string) (domain.Site, error) {
	return readFrom(ctx, s, func(image *state) (domain.Site, error) {
		return findByID(image.catalog.sites, id)
	})
}

func (s *store) CreateFacility(ctx context.Context, value domain.Facility) error {
	return s.withWrite(ctx, func(image *state) error {
		if _, exists := image.catalog.facilities[value.ID]; exists {
			return domain.ErrDuplicate
		}
		if _, exists := image.catalog.sites[value.SiteID]; !exists {
			return domain.ErrNotFound
		}
		for _, current := range image.catalog.facilities {
			if current.SiteID == value.SiteID && current.Code == value.Code {
				return domain.ErrDuplicate
			}
		}
		image.catalog.facilities[value.ID] = value
		return nil
	})
}

func (s *store) GetFacility(ctx context.Context, id string) (domain.Facility, error) {
	return readFrom(ctx, s, func(image *state) (domain.Facility, error) {
		return findByID(image.catalog.facilities, id)
	})
}

func (s *store) CreateRiskCategory(ctx context.Context, value domain.RiskCategory) error {
	return s.withWrite(ctx, func(image *state) error {
		if _, exists := image.catalog.categories[value.ID]; exists {
			return domain.ErrDuplicate
		}
		for _, current := range image.catalog.categories {
			if current.Code == value.Code {
				return domain.ErrDuplicate
			}
		}
		image.catalog.categories[value.ID] = value
		return nil
	})
}

func (s *store) GetRiskCategory(ctx context.Context, id string) (domain.RiskCategory, error) {
	return readFrom(ctx, s, func(image *state) (domain.RiskCategory, error) {
		return findByID(image.catalog.categories, id)
	})
}

func (s *store) SaveRule(ctx context.Context, value domain.EscalationRule, expected int64) error {
	return s.withWrite(ctx, func(image *state) error {
		versions := image.catalog.rules[value.ID]
		if expected == 0 {
			if len(versions) > 0 {
				return domain.ErrDuplicate
			}
		} else {
			if len(versions) == 0 {
				return domain.ErrNotFound
			}
			if versions[len(versions)-1].Version != expected {
				return domain.ErrConflict
			}
		}
		image.catalog.rules[value.ID] = append(versions, value)
		return nil
	})
}

func (s *store) GetRule(ctx context.Context, id string) (domain.EscalationRule, error) {
	return readFrom(ctx, s, func(image *state) (domain.EscalationRule, error) {
		versions := image.catalog.rules[id]
		if len(versions) == 0 {
			return domain.EscalationRule{}, domain.ErrNotFound
		}
		return versions[len(versions)-1], nil
	})
}

func (s *store) ListActiveRules(ctx context.Context) ([]domain.EscalationRule, error) {
	return readFrom(ctx, s, func(image *state) ([]domain.EscalationRule, error) {
		result := make([]domain.EscalationRule, 0)
		for _, versions := range image.catalog.rules {
			if len(versions) > 0 && versions[len(versions)-1].Enabled {
				result = append(result, versions[len(versions)-1])
			}
		}
		sort.Slice(result, func(i, j int) bool { return result[i].Priority < result[j].Priority })
		return result, nil
	})
}

func (s *store) SaveInspection(ctx context.Context, value domain.Inspection) error {
	return s.withWrite(ctx, func(image *state) error {
		if _, exists := image.hazards.inspections[value.ID]; exists {
			return domain.ErrDuplicate
		}
		image.hazards.inspections[value.ID] = cloneInspection(value)
		return nil
	})
}

func (s *store) SaveHazard(ctx context.Context, value domain.Hazard) error {
	return s.withWrite(ctx, func(image *state) error {
		if _, exists := image.hazards.hazards[value.ID]; exists {
			return domain.ErrDuplicate
		}
		if value.IdempotencyKey != "" {
			if _, exists := image.hazards.idempotency[value.IdempotencyKey]; exists {
				return domain.ErrDuplicate
			}
			image.hazards.idempotency[value.IdempotencyKey] = value.ID
		}
		image.hazards.hazards[value.ID] = cloneHazard(value)
		return nil
	})
}

func (s *store) UpdateHazard(ctx context.Context, value domain.Hazard, expected int64) error {
	return s.withWrite(ctx, func(image *state) error {
		current, ok := image.hazards.hazards[value.ID]
		if !ok {
			return domain.ErrNotFound
		}
		if current.Version != expected {
			return domain.ErrConflict
		}
		if current.IdempotencyKey != value.IdempotencyKey {
			return fmt.Errorf("idempotency key is immutable")
		}
		image.hazards.hazards[value.ID] = cloneHazard(value)
		return nil
	})
}

func (s *store) GetHazard(ctx context.Context, id string) (domain.Hazard, error) {
	return readFrom(ctx, s, func(image *state) (domain.Hazard, error) {
		value, err := findByID(image.hazards.hazards, id)
		if err != nil {
			return domain.Hazard{}, err
		}
		return cloneHazard(value), nil
	})
}

func (s *store) FindHazardByIdempotency(ctx context.Context, key string) (domain.Hazard, error) {
	return readFrom(ctx, s, func(image *state) (domain.Hazard, error) {
		id, ok := image.hazards.idempotency[key]
		if !ok {
			return domain.Hazard{}, domain.ErrNotFound
		}
		value, err := findByID(image.hazards.hazards, id)
		if err != nil {
			return domain.Hazard{}, err
		}
		return cloneHazard(value), nil
	})
}

func (s *store) SaveEvidence(ctx context.Context, value domain.Evidence) error {
	return s.withWrite(ctx, func(image *state) error {
		if _, ok := image.hazards.hazards[value.HazardID]; !ok {
			return domain.ErrNotFound
		}
		for _, current := range image.hazards.evidence[value.HazardID] {
			if current.ID == value.ID {
				return domain.ErrDuplicate
			}
		}
		image.hazards.evidence[value.HazardID] = append(image.hazards.evidence[value.HazardID], value)
		return nil
	})
}

func (s *store) ListEvidence(ctx context.Context, hazardID string) ([]domain.Evidence, error) {
	return readFrom(ctx, s, func(image *state) ([]domain.Evidence, error) {
		if _, ok := image.hazards.hazards[hazardID]; !ok {
			return nil, domain.ErrNotFound
		}
		return append([]domain.Evidence(nil), image.hazards.evidence[hazardID]...), nil
	})
}

func (s *store) SaveQueueEntry(ctx context.Context, value domain.QueueEntry, expected int64) error {
	return s.withWrite(ctx, func(image *state) error {
		return saveVersioned(image.hazards.queues, value.HazardID, value, expected, func(current domain.QueueEntry) int64 {
			return current.Version
		})
	})
}

func (s *store) GetQueueEntry(ctx context.Context, hazardID string) (domain.QueueEntry, error) {
	return readFrom(ctx, s, func(image *state) (domain.QueueEntry, error) {
		return findByID(image.hazards.queues, hazardID)
	})
}

func (s *store) ListQueue(ctx context.Context, filter domain.QueueFilter, now time.Time) (domain.Page[domain.Hazard], error) {
	return readFrom(ctx, s, func(image *state) (domain.Page[domain.Hazard], error) {
		rows := make([]domain.Hazard, 0)
		for id, queue := range image.hazards.queues {
			hazard := image.hazards.hazards[id]
			if filter.Queue != "" && queue.Queue != filter.Queue {
				continue
			}
			if filter.OwnerID != "" && queue.OwnerID != filter.OwnerID {
				continue
			}
			if filter.State != "" && hazard.State != filter.State {
				continue
			}
			if filter.RiskLevel != "" && hazard.RiskLevel != filter.RiskLevel {
				continue
			}
			if filter.SiteID != "" && hazard.SiteID != filter.SiteID {
				continue
			}
			if filter.OverdueOnly && !queue.Overdue(now) {
				continue
			}
			rows = append(rows, cloneHazard(hazard))
		}
		sort.SliceStable(rows, func(i, j int) bool {
			var less bool
			switch filter.Sort {
			case "created_at":
				less = rows[i].CreatedAt.Before(rows[j].CreatedAt)
			case "updated_at":
				less = rows[i].UpdatedAt.Before(rows[j].UpdatedAt)
			case "risk_score":
				less = rows[i].RiskScore < rows[j].RiskScore
			default:
				less = rows[i].DueAt.Before(rows[j].DueAt)
			}
			if filter.Descending {
				return !less
			}
			return less
		})
		total := len(rows)
		start := (filter.Page - 1) * filter.PageSize
		if start > total {
			start = total
		}
		end := min(start+filter.PageSize, total)
		return domain.NewPage(rows[start:end], filter.Page, filter.PageSize, total), nil
	})
}

func (s *store) ListDueQueueEntries(ctx context.Context, now time.Time, limit int) ([]domain.QueueEntry, error) {
	return readFrom(ctx, s, func(image *state) ([]domain.QueueEntry, error) {
		rows := make([]domain.QueueEntry, 0)
		for _, entry := range image.hazards.queues {
			if entry.NotificationDue(now) {
				rows = append(rows, entry)
			}
		}
		sort.Slice(rows, func(i, j int) bool { return rows[i].NextNotifyAt.Before(rows[j].NextNotifyAt) })
		if len(rows) > limit {
			rows = rows[:limit]
		}
		return rows, nil
	})
}

func (s *store) SaveRemediationPlan(ctx context.Context, value domain.RemediationPlan, expected int64) error {
	return s.withWrite(ctx, func(image *state) error {
		return saveVersioned(image.hazards.plans, value.HazardID, clonePlan(value), expected, func(current domain.RemediationPlan) int64 {
			return current.Version
		})
	})
}

func (s *store) GetRemediationPlan(ctx context.Context, hazardID string) (domain.RemediationPlan, error) {
	return readFrom(ctx, s, func(image *state) (domain.RemediationPlan, error) {
		value, err := findByID(image.hazards.plans, hazardID)
		if err != nil {
			return domain.RemediationPlan{}, err
		}
		return clonePlan(value), nil
	})
}

func (s *store) SaveReinspection(ctx context.Context, value domain.Reinspection) error {
	return s.withWrite(ctx, func(image *state) error {
		for _, current := range image.hazards.reinspections[value.HazardID] {
			if current.IdempotencyKey == value.IdempotencyKey {
				return domain.ErrDuplicate
			}
		}
		image.hazards.reinspections[value.HazardID] = append(image.hazards.reinspections[value.HazardID], value)
		return nil
	})
}

func (s *store) ListOpenHazardsForSite(ctx context.Context, siteID string) ([]domain.Hazard, error) {
	return readFrom(ctx, s, func(image *state) ([]domain.Hazard, error) {
		result := make([]domain.Hazard, 0)
		for _, hazard := range image.hazards.hazards {
			if hazard.SiteID == siteID && hazard.State != domain.StateClosed {
				result = append(result, cloneHazard(hazard))
			}
		}
		return result, nil
	})
}

func (s *store) SaveNotification(ctx context.Context, value domain.Notification) error {
	return s.withWrite(ctx, func(image *state) error {
		if _, exists := image.delivery.notifications[value.ID]; exists {
			return domain.ErrDuplicate
		}
		if _, exists := image.delivery.notificationByDedup[value.Delivery.DeduplicationKey]; exists {
			return domain.ErrDuplicate
		}
		image.delivery.notifications[value.ID] = value
		image.delivery.notificationByDedup[value.Delivery.DeduplicationKey] = value.ID
		return nil
	})
}

func (s *store) FindNotificationByDeduplication(ctx context.Context, key string) (domain.Notification, error) {
	return readFrom(ctx, s, func(image *state) (domain.Notification, error) {
		id, ok := image.delivery.notificationByDedup[key]
		if !ok {
			return domain.Notification{}, domain.ErrNotFound
		}
		return findByID(image.delivery.notifications, id)
	})
}

func (s *store) SaveNotificationAttempt(ctx context.Context, value domain.NotificationAttempt) error {
	return s.withWrite(ctx, func(image *state) error {
		if _, ok := image.delivery.notifications[value.NotificationID]; !ok {
			return domain.ErrNotFound
		}
		image.delivery.notificationAttempts = append(image.delivery.notificationAttempts, value)
		return nil
	})
}

func (s *store) AppendAudit(ctx context.Context, value domain.AuditEvent) error {
	return s.withWrite(ctx, func(image *state) error {
		for _, current := range image.audit.events {
			if current.ID == value.ID {
				return domain.ErrDuplicate
			}
		}
		image.audit.events = append(image.audit.events, cloneAudit(value))
		return nil
	})
}

func (s *store) ListAudit(ctx context.Context, hazardID string, page, pageSize int) (domain.Page[domain.AuditEvent], error) {
	return readFrom(ctx, s, func(image *state) (domain.Page[domain.AuditEvent], error) {
		if page < 1 {
			page = 1
		}
		if pageSize < 1 {
			pageSize = 20
		}
		if pageSize > 500 {
			pageSize = 500
		}
		rows := make([]domain.AuditEvent, 0)
		for _, event := range image.audit.events {
			if hazardID == "" || event.HazardID == hazardID {
				rows = append(rows, cloneAudit(event))
			}
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].OccurredAt.Before(rows[j].OccurredAt) })
		total := len(rows)
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := min(start+pageSize, total)
		return domain.NewPage(rows[start:end], page, pageSize, total), nil
	})
}

func (s *store) ListHazardsBetween(ctx context.Context, from, to time.Time) ([]domain.Hazard, error) {
	return readFrom(ctx, s, func(image *state) ([]domain.Hazard, error) {
		rows := make([]domain.Hazard, 0)
		for _, hazard := range image.hazards.hazards {
			if !hazard.CreatedAt.Before(from) && hazard.CreatedAt.Before(to) {
				rows = append(rows, cloneHazard(hazard))
			}
		}
		return rows, nil
	})
}

func (s *store) CheckReady(ctx context.Context) error {
	return s.withRead(ctx, func(image *state) error {
		if image == nil {
			return errors.New("memory repository is not initialized")
		}
		return ctx.Err()
	})
}

func cloneHazard(value domain.Hazard) domain.Hazard {
	value.ImpactScopes = append([]string(nil), value.ImpactScopes...)
	value.MatchedRuleEvidence = append([]string(nil), value.MatchedRuleEvidence...)
	if value.PassedReinspectionAt != nil {
		copy := *value.PassedReinspectionAt
		value.PassedReinspectionAt = &copy
	}
	return value
}

func cloneInspection(value domain.Inspection) domain.Inspection {
	value.Items = append([]domain.InspectionResult(nil), value.Items...)
	for index := range value.Items {
		value.Items[index].ImpactScopes = append([]string(nil), value.Items[index].ImpactScopes...)
	}
	return value
}

func clonePlan(value domain.RemediationPlan) domain.RemediationPlan {
	value.Tasks = append([]domain.RemediationTask(nil), value.Tasks...)
	for index := range value.Tasks {
		value.Tasks[index].EvidenceIDs = append([]string(nil), value.Tasks[index].EvidenceIDs...)
	}
	return value
}

func cloneAudit(value domain.AuditEvent) domain.AuditEvent {
	value.SensitiveKeys = append([]string(nil), value.SensitiveKeys...)
	if value.Details != nil {
		details := value.Details
		value.Details = make(map[string]any, len(details))
		for key, item := range details {
			value.Details[key] = item
		}
	}
	return value
}

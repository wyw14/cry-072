package application

import (
	"context"
	"fmt"

	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
)

type CatalogService struct {
	repository Repository
	clock      platform.TimeSource
	ids        platform.IDGenerator
}

func NewCatalogService(repository Repository, clock platform.TimeSource, ids platform.IDGenerator) *CatalogService {
	return &CatalogService{repository: repository, clock: clock, ids: ids}
}

func (s *CatalogService) CreateSite(ctx context.Context, site domain.Site, meta RequestMeta) (domain.Site, error) {
	if site.ID == "" {
		site.ID = s.ids.New("site")
	}
	if site.State == "" {
		site.State = domain.SiteActive
	}
	if err := site.Validate(); err != nil {
		return domain.Site{}, err
	}
	now := s.clock.Now()
	site.Version = 1
	site.CreatedAt = now
	site.UpdatedAt = now
	err := commitAudited(ctx, s.repository, auditedChange{
		operation: "create site",
		apply:     func(store Store) error { return store.CreateSite(ctx, site) },
		event: domain.AuditEvent{
			ID: s.ids.New("audit"), SiteID: site.ID, EventType: "site.created",
			ActorID: meta.ActorID, ActorRole: meta.Role, RequestID: meta.RequestID,
			Details: map[string]any{"code": site.Code, "name": site.Name}, OccurredAt: now,
		},
	})
	if err != nil {
		return domain.Site{}, err
	}
	return site, nil
}

func (s *CatalogService) CreateFacility(ctx context.Context, facility domain.Facility, meta RequestMeta) (domain.Facility, error) {
	if facility.ID == "" {
		facility.ID = s.ids.New("facility")
	}
	if err := facility.Validate(); err != nil {
		return domain.Facility{}, err
	}
	if _, err := s.repository.GetSite(ctx, facility.SiteID); err != nil {
		return domain.Facility{}, fmt.Errorf("load site: %w", err)
	}
	now := s.clock.Now()
	facility.Version = 1
	facility.CreatedAt = now
	facility.UpdatedAt = now
	if err := s.repository.CreateFacility(ctx, facility); err != nil {
		return domain.Facility{}, fmt.Errorf("create facility: %w", err)
	}
	return facility, nil
}

func (s *CatalogService) CreateRiskCategory(ctx context.Context, category domain.RiskCategory) (domain.RiskCategory, error) {
	if category.ID == "" {
		category.ID = s.ids.New("risk")
	}
	if err := category.Validate(); err != nil {
		return domain.RiskCategory{}, err
	}
	category.Version = 1
	category.CreatedAt = s.clock.Now()
	if err := s.repository.CreateRiskCategory(ctx, category); err != nil {
		return domain.RiskCategory{}, fmt.Errorf("create risk category: %w", err)
	}
	return category, nil
}

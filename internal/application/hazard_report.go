package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/service"
)

type EvidenceInput struct {
	Kind        string `json:"kind"`
	FileName    string `json:"file_name"`
	ContentType string `json:"content_type"`
	StorageKey  string `json:"storage_key"`
	SizeBytes   int64  `json:"size_bytes"`
	Description string `json:"description"`
}

type ReportHazardInput struct {
	InspectionID     string          `json:"inspection_id"`
	SiteID           string          `json:"site_id" validate:"required"`
	FacilityID       string          `json:"facility_id" validate:"required"`
	InspectionItemID string          `json:"inspection_item_id" validate:"required"`
	RiskCategoryID   string          `json:"risk_category_id" validate:"required"`
	Title            string          `json:"title" validate:"required,max=160"`
	Description      string          `json:"description" validate:"required,max=4000"`
	Severity         int             `json:"severity" validate:"min=1,max=5"`
	Likelihood       int             `json:"likelihood" validate:"min=1,max=5"`
	ImpactScopes     []string        `json:"impact_scopes"`
	InitialAction    string          `json:"initial_action" validate:"required,max=1000"`
	IdempotencyKey   string          `json:"idempotency_key" validate:"required,max=120"`
	Evidence         []EvidenceInput `json:"evidence"`
}

// reportSession keeps every decision that contributes to one report inside the
// same repository transaction. It prevents a risk score or rule snapshot from
// being calculated against catalog data that changes before persistence.
type reportSession struct {
	service    *HazardService
	store      Store
	input      ReportHazardInput
	meta       RequestMeta
	observedAt time.Time

	site        domain.Site
	facility    domain.Facility
	category    domain.RiskCategory
	assessment  service.RiskAssessment
	ruleMatch   *domain.RuleMatch
	matchedRule *domain.EscalationRule
}

func (s *HazardService) Report(ctx context.Context, input ReportHazardInput, meta RequestMeta) (domain.Hazard, error) {
	input = normalizeReport(input)
	if err := validateReportEvidence(input.Evidence); err != nil {
		return domain.Hazard{}, err
	}
	if input.IdempotencyKey == "" {
		return domain.Hazard{}, domain.NewValidationError("idempotency_key", "幂等键不能为空")
	}

	var result domain.Hazard
	err := s.repository.WithinTx(ctx, func(store Store) error {
		replayed, err := store.FindHazardByIdempotency(ctx, input.IdempotencyKey)
		if err == nil {
			result = replayed
			return nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return fmt.Errorf("check report replay: %w", err)
		}

		session := reportSession{
			service: s, store: store, input: input, meta: meta, observedAt: s.clock.Now(),
		}
		if err := session.resolveCatalog(ctx); err != nil {
			return err
		}
		if err := session.assessAndMatch(ctx); err != nil {
			return err
		}
		result, err = session.materialize()
		if err != nil {
			return err
		}
		return session.commit(ctx, result)
	})
	if err == nil {
		return result, nil
	}
	if errors.Is(err, domain.ErrDuplicate) {
		if replayed, lookupErr := s.repository.FindHazardByIdempotency(ctx, input.IdempotencyKey); lookupErr == nil {
			return replayed, nil
		}
	}
	return domain.Hazard{}, fmt.Errorf("report hazard: %w", err)
}

func normalizeReport(input ReportHazardInput) ReportHazardInput {
	input.InspectionID = strings.TrimSpace(input.InspectionID)
	input.SiteID = strings.TrimSpace(input.SiteID)
	input.FacilityID = strings.TrimSpace(input.FacilityID)
	input.InspectionItemID = strings.TrimSpace(input.InspectionItemID)
	input.RiskCategoryID = strings.TrimSpace(input.RiskCategoryID)
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.InitialAction = strings.TrimSpace(input.InitialAction)
	input.IdempotencyKey = strings.TrimSpace(input.IdempotencyKey)
	input.Evidence = normalizeEvidence(input.Evidence)
	return input
}

// normalizeEvidence collapses repeated client-side file names before the
// report is scored and persisted. Mobile inspectors commonly retry uploads,
// so the ingestion path treats the visible name as the logical attachment.
func normalizeEvidence(items []EvidenceInput) []EvidenceInput {
	normalized := make([]EvidenceInput, 0, len(items))
	positionByName := make(map[string]int, len(items))
	for _, raw := range items {
		item := trimEvidence(raw)
		identity := strings.ToLower(item.FileName)
		if identity == "" {
			normalized = append(normalized, item)
			continue
		}
		position, exists := positionByName[identity]
		if !exists {
			positionByName[identity] = len(normalized)
			normalized = append(normalized, item)
			continue
		}
		normalized[position] = mergeEvidence(normalized[position], item)
	}
	return normalized
}

func trimEvidence(item EvidenceInput) EvidenceInput {
	item.Kind = strings.TrimSpace(item.Kind)
	item.FileName = strings.TrimSpace(item.FileName)
	item.ContentType = strings.TrimSpace(item.ContentType)
	item.StorageKey = strings.TrimSpace(item.StorageKey)
	item.Description = strings.TrimSpace(item.Description)
	return item
}

func mergeEvidence(current, retry EvidenceInput) EvidenceInput {
	if current.Kind == "" {
		current.Kind = retry.Kind
	}
	if current.ContentType == "" {
		current.ContentType = retry.ContentType
	}
	if current.StorageKey == "" {
		current.StorageKey = retry.StorageKey
	}
	if retry.SizeBytes > current.SizeBytes {
		current.SizeBytes = retry.SizeBytes
	}
	current.Description = mergeEvidenceDescription(current.Description, retry.Description)
	return current
}

func mergeEvidenceDescription(current, retry string) string {
	switch {
	case current == "":
		return retry
	case retry == "" || retry == current:
		return current
	default:
		return current + "；" + retry
	}
}

func validateReportEvidence(items []EvidenceInput) error {
	seen := make(map[string]struct{}, len(items))
	for _, item := range items {
		if item.StorageKey == "" || item.FileName == "" || item.SizeBytes <= 0 {
			return domain.NewValidationError("evidence", "证据文件信息不完整")
		}
		if _, duplicate := seen[item.StorageKey]; duplicate {
			return domain.NewValidationError("evidence", "同一证据文件不能重复关联")
		}
		seen[item.StorageKey] = struct{}{}
	}
	return nil
}

func (r *reportSession) resolveCatalog(ctx context.Context) error {
	var err error
	r.site, err = r.store.GetSite(ctx, r.input.SiteID)
	if err != nil {
		return fmt.Errorf("load report site: %w", err)
	}
	r.facility, err = r.store.GetFacility(ctx, r.input.FacilityID)
	if err != nil {
		return fmt.Errorf("load report facility: %w", err)
	}
	if r.facility.SiteID != r.site.ID || !r.facility.Enabled {
		return domain.NewValidationError("facility_id", "设施不属于当前场地或已停用")
	}
	r.category, err = r.store.GetRiskCategory(ctx, r.input.RiskCategoryID)
	if err != nil {
		return fmt.Errorf("load report risk category: %w", err)
	}
	return nil
}

func (r *reportSession) assessAndMatch(ctx context.Context) error {
	var err error
	r.assessment, err = r.service.calculator.Calculate(
		r.category, r.input.Severity, r.input.Likelihood, r.input.ImpactScopes, r.facility.Critical,
	)
	if err != nil {
		return err
	}
	rules, err := r.store.ListActiveRules(ctx)
	if err != nil {
		return fmt.Errorf("load report escalation rules: %w", err)
	}
	preview := domain.Hazard{
		RiskCategoryID: r.category.ID,
		RiskScore:      r.assessment.Score,
		RiskLevel:      r.assessment.Level,
		ImpactScopes:   append([]string(nil), r.input.ImpactScopes...),
	}
	r.ruleMatch, err = r.service.rules.Evaluate(service.RuleInput{
		Hazard: preview, EvidenceCount: len(r.input.Evidence), CriticalFacility: r.facility.Critical,
	}, rules, r.observedAt)
	if err != nil || r.ruleMatch == nil {
		return err
	}
	for index := range rules {
		candidate := rules[index]
		if candidate.ID == r.ruleMatch.RuleID && candidate.Version == r.ruleMatch.RuleVersion {
			r.matchedRule = &candidate
			break
		}
	}
	return nil
}

func (r *reportSession) materialize() (domain.Hazard, error) {
	hazard := domain.Hazard{
		ID: r.service.ids.New("hazard"), InspectionID: r.input.InspectionID, SiteID: r.site.ID,
		FacilityID: r.facility.ID, InspectionItemID: r.input.InspectionItemID, RiskCategoryID: r.category.ID,
		Title: r.input.Title, Description: r.input.Description, Severity: r.input.Severity, Likelihood: r.input.Likelihood,
		RiskScore: r.assessment.Score, RiskLevel: r.assessment.Level,
		ImpactScopes: append([]string(nil), r.input.ImpactScopes...), InitialAction: r.input.InitialAction,
		IdempotencyKey: r.input.IdempotencyKey, State: domain.StatePending, Queue: domain.QueueNormal,
		Version: 1, CreatedAt: r.observedAt, UpdatedAt: r.observedAt,
	}
	if r.ruleMatch != nil {
		hazard.Queue = domain.QueueFocus
		hazard.Escalated = true
		hazard.EscalationReason = strings.Join(r.ruleMatch.Reasons, "; ")
		hazard.MatchedRuleID = r.ruleMatch.RuleID
		hazard.MatchedRuleVersion = r.ruleMatch.RuleVersion
		hazard.MatchedRuleEvidence = append([]string(nil), r.ruleMatch.Reasons...)
	}
	hazard.DueAt = r.service.deadlines.ForHazard(hazard, r.category, r.matchedRule, r.observedAt)
	return hazard, hazard.ValidateNew()
}

func (r *reportSession) commit(ctx context.Context, hazard domain.Hazard) error {
	queue := domain.QueueEntry{
		HazardID: hazard.ID, Queue: hazard.Queue, DueAt: hazard.DueAt, Version: 1, UpdatedAt: r.observedAt,
	}
	queue.NextNotifyAt = r.service.deadlines.NextNotification(queue, r.matchedRule, r.observedAt)
	if err := r.store.SaveHazard(ctx, hazard); err != nil {
		return err
	}
	if err := r.store.SaveQueueEntry(ctx, queue, 0); err != nil {
		return err
	}
	for _, item := range r.input.Evidence {
		evidence := domain.Evidence{
			ID: r.service.ids.New("evidence"), HazardID: hazard.ID, Kind: item.Kind, FileName: item.FileName,
			ContentType: item.ContentType, StorageKey: item.StorageKey, SizeBytes: item.SizeBytes,
			Description: item.Description, UploadedBy: r.meta.ActorID, CreatedAt: r.observedAt,
		}
		if err := r.store.SaveEvidence(ctx, evidence); err != nil {
			return err
		}
	}
	return r.store.AppendAudit(ctx, domain.AuditEvent{
		ID: r.service.ids.New("audit"), HazardID: hazard.ID, SiteID: hazard.SiteID,
		EventType: "hazard.reported", ActorID: r.meta.ActorID, ActorRole: r.meta.Role,
		RuleID: hazard.MatchedRuleID, RuleVersion: hazard.MatchedRuleVersion,
		CurrentState: hazard.State, RequestID: r.meta.RequestID, SensitiveKeys: []string{"idempotency_key"},
		Details: map[string]any{
			"risk_score": r.assessment.Score, "risk_level": r.assessment.Level, "queue": hazard.Queue,
			"risk_factors": r.assessment.Factors, "rule_evidence": hazard.MatchedRuleEvidence,
			"idempotency_key": r.input.IdempotencyKey,
		},
		OccurredAt: r.observedAt,
	})
}

package application

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
)

type ReportService struct {
	repository Repository
}

func NewReportService(repository Repository) *ReportService {
	return &ReportService{repository: repository}
}

func (s *ReportService) Timeline(ctx context.Context, hazardID string, page, pageSize int) (domain.Page[domain.AuditEvent], error) {
	if strings.TrimSpace(hazardID) == "" {
		return domain.Page[domain.AuditEvent]{}, domain.NewValidationError("hazard_id", "隐患标识不能为空")
	}
	result, err := s.repository.ListAudit(ctx, hazardID, page, pageSize)
	if err != nil {
		return domain.Page[domain.AuditEvent]{}, fmt.Errorf("list audit timeline: %w", err)
	}
	for index := range result.Items {
		result.Items[index].Details = result.Items[index].PublicDetails()
	}
	return result, nil
}

func (s *ReportService) ExportAuditCSV(ctx context.Context, hazardID string, writer io.Writer) error {
	if strings.TrimSpace(hazardID) == "" {
		return fmt.Errorf("hazard id is required")
	}
	page, err := s.repository.ListAudit(ctx, hazardID, 1, 500)
	if err != nil {
		return fmt.Errorf("list audit timeline: %w", err)
	}
	output := csv.NewWriter(writer)
	if err := output.Write([]string{
		"event_id", "hazard_id", "event_type", "actor_id", "actor_role", "rule_id",
		"rule_version", "previous_state", "current_state", "reason", "request_id", "occurred_at", "details",
	}); err != nil {
		return err
	}
	for _, event := range page.Items {
		record := auditCSVRecord(event)
		if err := output.Write(record); err != nil {
			return fmt.Errorf("write audit report: %w", err)
		}
	}
	output.Flush()
	if err := output.Error(); err != nil {
		return fmt.Errorf("flush audit report: %w", err)
	}
	return nil
}

func auditCSVRecord(event domain.AuditEvent) []string {
	return []string{
		event.ID, event.HazardID, event.EventType, event.ActorID, string(event.ActorRole), event.RuleID,
		strconv.FormatInt(event.RuleVersion, 10), string(event.PreviousState), string(event.CurrentState),
		event.Reason, event.RequestID, event.OccurredAt.UTC().Format(time.RFC3339Nano),
		encodeAuditDetails(event.Details),
	}
}

func encodeAuditDetails(details map[string]any) string {
	if len(details) == 0 {
		return "{}"
	}
	encoded, err := json.Marshal(normalizeAuditDetails(details))
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

func normalizeAuditDetails(details map[string]any) map[string]any {
	result := make(map[string]any, len(details))
	for key, value := range details {
		result[key] = normalizeAuditValue(value)
	}
	return result
}

func normalizeAuditValue(value any) any {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC().Format(time.RFC3339Nano)
	case fmt.Stringer:
		return typed.String()
	case []string:
		items := make([]string, len(typed))
		copy(items, typed)
		return items
	case map[string]string:
		items := make(map[string]string, len(typed))
		for key, item := range typed {
			items[key] = item
		}
		return items
	case map[string]any:
		return normalizeAuditDetails(typed)
	default:
		return value
	}
}

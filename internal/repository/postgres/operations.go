package postgres

import (
	"context"
	"fmt"

	"github.com/wyw14/cry-072/internal/domain"
)

func (s *store) SaveRemediationPlan(ctx context.Context, value domain.RemediationPlan, expected int64) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	if expected == 0 {
		_, err = s.query.Exec(bounded, `INSERT INTO remediation_plans(id,hazard_id,version,data) VALUES($1,$2,$3,$4)`, value.ID, value.HazardID, value.Version, payload)
		return translate(err)
	}
	tag, err := s.query.Exec(bounded, `UPDATE remediation_plans SET version=$2,data=$3 WHERE hazard_id=$1 AND version=$4`, value.HazardID, value.Version, payload, expected)
	if err != nil {
		return translate(err)
	}
	return ensureUpdated(tag)
}

func (s *store) GetRemediationPlan(ctx context.Context, hazardID string) (domain.RemediationPlan, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var payload []byte
	err := s.query.QueryRow(bounded, `SELECT data FROM remediation_plans WHERE hazard_id=$1`, hazardID).Scan(&payload)
	if err != nil {
		return domain.RemediationPlan{}, translate(err)
	}
	return decode[domain.RemediationPlan](payload)
}

func (s *store) SaveReinspection(ctx context.Context, value domain.Reinspection) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err = s.query.Exec(bounded, `INSERT INTO reinspections(id,hazard_id,idempotency_key,completed_at,data) VALUES($1,$2,$3,$4,$5)`, value.ID, value.HazardID, value.IdempotencyKey, value.CompletedAt, payload)
	return translate(err)
}

func (s *store) SaveNotification(ctx context.Context, value domain.Notification) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err = s.query.Exec(bounded, `INSERT INTO notifications(id,hazard_id,deduplication_key,sent_at,data) VALUES($1,$2,$3,$4,$5)`, value.ID, value.HazardID, value.Delivery.DeduplicationKey, nullableTime(value.Delivery.SentAt), payload)
	return translate(err)
}

func (s *store) FindNotificationByDeduplication(ctx context.Context, key string) (domain.Notification, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var payload []byte
	err := s.query.QueryRow(bounded, `SELECT data FROM notifications WHERE deduplication_key=$1`, key).Scan(&payload)
	if err != nil {
		return domain.Notification{}, translate(err)
	}
	return decode[domain.Notification](payload)
}

func (s *store) SaveNotificationAttempt(ctx context.Context, value domain.NotificationAttempt) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err = s.query.Exec(bounded, `INSERT INTO notification_attempts(notification_id,attempted_at,data) VALUES($1,$2,$3)`, value.NotificationID, value.AttemptedAt, payload)
	return translate(err)
}

func (s *store) AppendAudit(ctx context.Context, value domain.AuditEvent) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err = s.query.Exec(bounded, `INSERT INTO audit_events(id,hazard_id,site_id,event_type,occurred_at,data) VALUES($1,$2,$3,$4,$5,$6)`, value.ID, nullString(value.HazardID), nullString(value.SiteID), value.EventType, value.OccurredAt, payload)
	return translate(err)
}

func (s *store) ListAudit(ctx context.Context, hazardID string, page, pageSize int) (domain.Page[domain.AuditEvent], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	where := "TRUE"
	args := []any{}
	if hazardID != "" {
		where = "hazard_id=$1"
		args = append(args, hazardID)
	}
	var total int
	if err := s.query.QueryRow(bounded, `SELECT count(*) FROM audit_events WHERE `+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.AuditEvent]{}, translate(err)
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.query.Query(bounded, fmt.Sprintf(`SELECT data FROM audit_events WHERE %s ORDER BY occurred_at,id LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return domain.Page[domain.AuditEvent]{}, translate(err)
	}
	defer rows.Close()
	values := make([]domain.AuditEvent, 0, pageSize)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return domain.Page[domain.AuditEvent]{}, err
		}
		value, err := decode[domain.AuditEvent](payload)
		if err != nil {
			return domain.Page[domain.AuditEvent]{}, err
		}
		value.Details = value.PublicDetails()
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return domain.Page[domain.AuditEvent]{}, err
	}
	return domain.NewPage(values, page, pageSize, total), nil
}

func (s *store) CheckReady(ctx context.Context) error {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var value int
	if err := s.query.QueryRow(bounded, `SELECT 1`).Scan(&value); err != nil {
		return fmt.Errorf("postgres readiness: %w", err)
	}
	if value != 1 {
		return fmt.Errorf("postgres readiness returned %d", value)
	}
	return nil
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

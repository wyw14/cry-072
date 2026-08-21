package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
)

type rowScanner interface {
	Scan(...any) error
}

func (s *store) CreateSite(ctx context.Context, value domain.Site) error {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err := s.query.Exec(bounded, `
		INSERT INTO sites(id,code,name,region,state,version,created_at,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		value.ID, value.Code, value.Name, value.Region, value.State, value.Version,
		value.CreatedAt, value.UpdatedAt,
	)
	return translate(err)
}

func (s *store) UpdateSite(ctx context.Context, value domain.Site, expected int64) error {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	tag, err := s.query.Exec(bounded, `
		UPDATE sites
		SET code=$2,name=$3,region=$4,state=$5,version=$6,updated_at=$7
		WHERE id=$1 AND version=$8`,
		value.ID, value.Code, value.Name, value.Region, value.State, value.Version,
		value.UpdatedAt, expected,
	)
	if err != nil {
		return translate(err)
	}
	return ensureUpdated(tag)
}

func scanSite(row rowScanner) (domain.Site, error) {
	var value domain.Site
	err := row.Scan(
		&value.ID, &value.Code, &value.Name, &value.Region, &value.State,
		&value.Version, &value.CreatedAt, &value.UpdatedAt,
	)
	return value, translate(err)
}

func (s *store) GetSite(ctx context.Context, id string) (domain.Site, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	return scanSite(s.query.QueryRow(bounded, `
		SELECT id,code,name,region,state,version,created_at,updated_at
		FROM sites WHERE id=$1`, id))
}

func (s *store) CreateFacility(ctx context.Context, value domain.Facility) error {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err := s.query.Exec(bounded, `
		INSERT INTO facilities(
			id,site_id,code,name,kind,critical,enabled,version,created_at,updated_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`,
		value.ID, value.SiteID, value.Code, value.Name, value.Kind, value.Critical,
		value.Enabled, value.Version, value.CreatedAt, value.UpdatedAt,
	)
	return translate(err)
}

func (s *store) GetFacility(ctx context.Context, id string) (domain.Facility, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var value domain.Facility
	err := s.query.QueryRow(bounded, `
		SELECT id,site_id,code,name,kind,critical,enabled,version,created_at,updated_at
		FROM facilities WHERE id=$1`, id).Scan(
		&value.ID, &value.SiteID, &value.Code, &value.Name, &value.Kind, &value.Critical,
		&value.Enabled, &value.Version, &value.CreatedAt, &value.UpdatedAt,
	)
	return value, translate(err)
}

func (s *store) CreateRiskCategory(ctx context.Context, value domain.RiskCategory) error {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err := s.query.Exec(bounded, `
		INSERT INTO risk_categories(
			id,code,name,base_weight,default_sla_minutes,description,version,created_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`,
		value.ID, value.Code, value.Name, value.BaseWeight, value.DefaultSLA,
		value.Description, value.Version, value.CreatedAt,
	)
	return translate(err)
}

func (s *store) GetRiskCategory(ctx context.Context, id string) (domain.RiskCategory, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var value domain.RiskCategory
	err := s.query.QueryRow(bounded, `
		SELECT id,code,name,base_weight,default_sla_minutes,description,version,created_at
		FROM risk_categories WHERE id=$1`, id).Scan(
		&value.ID, &value.Code, &value.Name, &value.BaseWeight, &value.DefaultSLA,
		&value.Description, &value.Version, &value.CreatedAt,
	)
	return value, translate(err)
}

func (s *store) SaveRule(ctx context.Context, value domain.EscalationRule, expected int64) error {
	condition, err := encode(value.Condition)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	if expected > 0 {
		var latest int64
		err := s.query.QueryRow(bounded, `
			SELECT version FROM escalation_rule_versions
			WHERE rule_id=$1 ORDER BY version DESC LIMIT 1 FOR UPDATE`, value.ID).Scan(&latest)
		if err != nil {
			return translate(err)
		}
		if latest != expected {
			return domain.ErrConflict
		}
	}
	_, err = s.query.Exec(bounded, `
		INSERT INTO escalation_rule_versions(
			rule_id,version,name,description,enabled,priority,condition,
			focus_sla_millis,notify_after_millis,renotify_every_millis,
			created_by,created_at,updated_at
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`,
		value.ID, value.Version, value.Name, value.Description, value.Enabled,
		value.Priority, condition, value.FocusSLA.Milliseconds(), value.NotifyAfter.Milliseconds(),
		value.ReNotifyEvery.Milliseconds(), value.CreatedBy, value.CreatedAt, value.UpdatedAt,
	)
	return translate(err)
}

func scanRule(row rowScanner) (domain.EscalationRule, error) {
	var value domain.EscalationRule
	var condition []byte
	var focusMillis, notifyMillis, renotifyMillis int64
	err := row.Scan(
		&value.ID, &value.Version, &value.Name, &value.Description, &value.Enabled,
		&value.Priority, &condition, &focusMillis, &notifyMillis, &renotifyMillis,
		&value.CreatedBy, &value.CreatedAt, &value.UpdatedAt,
	)
	if err != nil {
		return domain.EscalationRule{}, translate(err)
	}
	value.Condition, err = decode[domain.RuleCondition](condition)
	if err != nil {
		return domain.EscalationRule{}, err
	}
	value.FocusSLA = time.Duration(focusMillis) * time.Millisecond
	value.NotifyAfter = time.Duration(notifyMillis) * time.Millisecond
	value.ReNotifyEvery = time.Duration(renotifyMillis) * time.Millisecond
	return value, nil
}

const ruleColumns = `
	rule_id,version,name,description,enabled,priority,condition,
	focus_sla_millis,notify_after_millis,renotify_every_millis,
	created_by,created_at,updated_at`

func (s *store) GetRule(ctx context.Context, id string) (domain.EscalationRule, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	return scanRule(s.query.QueryRow(bounded, `SELECT `+ruleColumns+`
		FROM escalation_rule_versions
		WHERE rule_id=$1 ORDER BY version DESC LIMIT 1`, id))
}

func (s *store) ListActiveRules(ctx context.Context) ([]domain.EscalationRule, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	rows, err := s.query.Query(bounded, `SELECT DISTINCT ON(rule_id) `+ruleColumns+`
		FROM escalation_rule_versions ORDER BY rule_id,version DESC`)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	values := make([]domain.EscalationRule, 0)
	for rows.Next() {
		value, err := scanRule(rows)
		if err != nil {
			return nil, fmt.Errorf("scan escalation rule: %w", err)
		}
		if value.Enabled {
			values = append(values, value)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate escalation rules: %w", err)
	}
	return values, nil
}

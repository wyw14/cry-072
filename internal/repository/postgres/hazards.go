package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
)

func (s *store) SaveInspection(ctx context.Context, value domain.Inspection) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err = s.query.Exec(bounded, `INSERT INTO inspections(id,site_id,facility_id,idempotency_key,created_at,data) VALUES($1,$2,$3,$4,$5,$6)`, value.ID, value.SiteID, value.FacilityID, value.IdempotencyKey, value.CreatedAt, payload)
	return translate(err)
}

func (s *store) SaveHazard(ctx context.Context, value domain.Hazard) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err = s.query.Exec(bounded, `INSERT INTO hazards(id,idempotency_key,site_id,queue,state,risk_level,risk_score,due_at,created_at,updated_at,version,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, value.ID, value.IdempotencyKey, value.SiteID, value.Queue, value.State, value.RiskLevel, value.RiskScore, value.DueAt, value.CreatedAt, value.UpdatedAt, value.Version, payload)
	return translate(err)
}

func (s *store) UpdateHazard(ctx context.Context, value domain.Hazard, expected int64) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	tag, err := s.query.Exec(bounded, `UPDATE hazards SET queue=$2,state=$3,risk_level=$4,risk_score=$5,due_at=$6,updated_at=$7,version=$8,data=$9 WHERE id=$1 AND version=$10`, value.ID, value.Queue, value.State, value.RiskLevel, value.RiskScore, value.DueAt, value.UpdatedAt, value.Version, payload, expected)
	if err != nil {
		return translate(err)
	}
	return ensureUpdated(tag)
}

func (s *store) GetHazard(ctx context.Context, id string) (domain.Hazard, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var payload []byte
	err := s.query.QueryRow(bounded, `SELECT data FROM hazards WHERE id=$1`, id).Scan(&payload)
	if err != nil {
		return domain.Hazard{}, translate(err)
	}
	return decode[domain.Hazard](payload)
}

func (s *store) FindHazardByIdempotency(ctx context.Context, key string) (domain.Hazard, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var payload []byte
	err := s.query.QueryRow(bounded, `SELECT data FROM hazards WHERE idempotency_key=$1`, key).Scan(&payload)
	if err != nil {
		return domain.Hazard{}, translate(err)
	}
	return decode[domain.Hazard](payload)
}

func (s *store) SaveEvidence(ctx context.Context, value domain.Evidence) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	_, err = s.query.Exec(bounded, `INSERT INTO evidence(id,hazard_id,storage_key,created_at,data) VALUES($1,$2,$3,$4,$5)`, value.ID, value.HazardID, value.StorageKey, value.CreatedAt, payload)
	return translate(err)
}

func (s *store) ListEvidence(ctx context.Context, hazardID string) ([]domain.Evidence, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	rows, err := s.query.Query(bounded, `SELECT data FROM evidence WHERE hazard_id=$1 ORDER BY created_at,id`, hazardID)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	values := make([]domain.Evidence, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		value, err := decode[domain.Evidence](payload)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *store) SaveQueueEntry(ctx context.Context, value domain.QueueEntry, expected int64) error {
	payload, err := encode(value)
	if err != nil {
		return err
	}
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	if expected == 0 {
		_, err = s.query.Exec(bounded, `INSERT INTO queue_entries(hazard_id,queue,owner_id,due_at,next_notify_at,acknowledged_at,version,data) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, value.HazardID, value.Queue, value.OwnerID, value.DueAt, nullableTime(value.NextNotifyAt), nullableTime(value.AcknowledgedAt), value.Version, payload)
		return translate(err)
	}
	tag, err := s.query.Exec(bounded, `UPDATE queue_entries SET queue=$2,owner_id=$3,due_at=$4,next_notify_at=$5,acknowledged_at=$6,version=$7,data=$8 WHERE hazard_id=$1 AND version=$9`, value.HazardID, value.Queue, value.OwnerID, value.DueAt, nullableTime(value.NextNotifyAt), nullableTime(value.AcknowledgedAt), value.Version, payload, expected)
	if err != nil {
		return translate(err)
	}
	return ensureUpdated(tag)
}

func (s *store) GetQueueEntry(ctx context.Context, hazardID string) (domain.QueueEntry, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var payload []byte
	err := s.query.QueryRow(bounded, `SELECT data FROM queue_entries WHERE hazard_id=$1`, hazardID).Scan(&payload)
	if err != nil {
		return domain.QueueEntry{}, translate(err)
	}
	return decode[domain.QueueEntry](payload)
}

func (s *store) ListQueue(ctx context.Context, filter domain.QueueFilter, now time.Time) (domain.Page[domain.Hazard], error) {
	filter = filter.Normalize()
	conditions := []string{"1=1"}
	args := make([]any, 0, 8)
	add := func(expression string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(expression, len(args)))
	}
	if filter.Queue != "" {
		add("q.queue=$%d", filter.Queue)
	}
	if filter.OwnerID != "" {
		add("q.owner_id=$%d", filter.OwnerID)
	}
	if filter.State != "" {
		add("h.state=$%d", filter.State)
	}
	if filter.RiskLevel != "" {
		add("h.risk_level=$%d", filter.RiskLevel)
	}
	if filter.SiteID != "" {
		add("h.site_id=$%d", filter.SiteID)
	}
	if filter.OverdueOnly {
		add("q.due_at<$%d", now)
	}
	where := strings.Join(conditions, " AND ")
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	var total int
	if err := s.query.QueryRow(bounded, `SELECT count(*) FROM hazards h JOIN queue_entries q ON q.hazard_id=h.id WHERE `+where, args...).Scan(&total); err != nil {
		return domain.Page[domain.Hazard]{}, translate(err)
	}
	sortColumn := map[string]string{"created_at": "h.created_at", "updated_at": "h.updated_at", "due_at": "q.due_at", "risk_score": "h.risk_score"}[filter.Sort]
	direction := "ASC"
	if filter.Descending {
		direction = "DESC"
	}
	args = append(args, filter.PageSize, (filter.Page-1)*filter.PageSize)
	query := fmt.Sprintf(`SELECT h.data FROM hazards h JOIN queue_entries q ON q.hazard_id=h.id WHERE %s ORDER BY %s %s,h.id LIMIT $%d OFFSET $%d`, where, sortColumn, direction, len(args)-1, len(args))
	rows, err := s.query.Query(bounded, query, args...)
	if err != nil {
		return domain.Page[domain.Hazard]{}, translate(err)
	}
	defer rows.Close()
	values := make([]domain.Hazard, 0, filter.PageSize)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return domain.Page[domain.Hazard]{}, err
		}
		value, err := decode[domain.Hazard](payload)
		if err != nil {
			return domain.Page[domain.Hazard]{}, err
		}
		values = append(values, value)
	}
	if err := rows.Err(); err != nil {
		return domain.Page[domain.Hazard]{}, err
	}
	return domain.NewPage(values, filter.Page, filter.PageSize, total), nil
}

func (s *store) ListDueQueueEntries(ctx context.Context, now time.Time, limit int) ([]domain.QueueEntry, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	rows, err := s.query.Query(bounded, `SELECT data FROM queue_entries WHERE acknowledged_at IS NULL AND next_notify_at<=$1 ORDER BY next_notify_at,hazard_id LIMIT $2 FOR UPDATE SKIP LOCKED`, now, limit)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	values := make([]domain.QueueEntry, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		value, err := decode[domain.QueueEntry](payload)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *store) ListOpenHazardsForSite(ctx context.Context, siteID string) ([]domain.Hazard, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	rows, err := s.query.Query(bounded, `SELECT data FROM hazards WHERE site_id=$1 AND state<>$2 ORDER BY created_at`, siteID, domain.StateClosed)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	values := make([]domain.Hazard, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		value, err := decode[domain.Hazard](payload)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func (s *store) ListHazardsBetween(ctx context.Context, from, to time.Time) ([]domain.Hazard, error) {
	bounded, cancel := s.bounded(ctx)
	defer cancel()
	rows, err := s.query.Query(bounded, `SELECT data FROM hazards WHERE created_at>=$1 AND created_at<$2 ORDER BY created_at,id`, from, to)
	if err != nil {
		return nil, translate(err)
	}
	defer rows.Close()
	values := make([]domain.Hazard, 0)
	for rows.Next() {
		var payload []byte
		if err := rows.Scan(&payload); err != nil {
			return nil, err
		}
		value, err := decode[domain.Hazard](payload)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return value
}

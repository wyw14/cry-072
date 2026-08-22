package application

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/wyw14/cry-072/internal/domain"
	"github.com/wyw14/cry-072/internal/platform"
)

type AnalyticsService struct {
	repository Repository
	clock      platform.TimeSource
}

func NewAnalyticsService(repository Repository, clock platform.TimeSource) *AnalyticsService {
	return &AnalyticsService{repository: repository, clock: clock}
}

func (s *AnalyticsService) Snapshot(ctx context.Context, from, to time.Time) (domain.AnalyticsSnapshot, error) {
	window, err := newAnalyticsWindow(from, to)
	if err != nil {
		return domain.AnalyticsSnapshot{}, err
	}
	hazards, err := s.repository.ListHazardsBetween(ctx, window.start, window.repositoryEnd())
	if err != nil {
		return domain.AnalyticsSnapshot{}, fmt.Errorf("list analytics hazards: %w", err)
	}
	normalRows, focusRows := window.split(hazards)
	distribution := map[domain.RiskLevel]int{
		domain.RiskLow: 0, domain.RiskMedium: 0, domain.RiskHigh: 0, domain.RiskCritical: 0,
	}
	bySite := make(map[string]int)
	for _, hazard := range hazards {
		if !window.contains(hazard.CreatedAt) {
			continue
		}
		distribution[hazard.RiskLevel]++
		bySite[hazard.SiteID]++
	}
	now := s.clock.Now()
	snapshot := domain.AnalyticsSnapshot{
		From: window.start, To: window.end,
		Normal:      calculateQueueMetrics(domain.QueueNormal, normalRows, now),
		Focus:       calculateQueueMetrics(domain.QueueFocus, focusRows, now),
		GeneratedAt: now,
	}
	for _, level := range []domain.RiskLevel{domain.RiskLow, domain.RiskMedium, domain.RiskHigh, domain.RiskCritical} {
		snapshot.RiskDistribution = append(snapshot.RiskDistribution, domain.RiskDistribution{Level: level, Count: distribution[level]})
	}
	mean := 0.0
	if len(bySite) > 0 {
		for _, count := range bySite {
			mean += float64(count)
		}
		mean /= float64(len(bySite))
	}
	for siteID, count := range bySite {
		if mean > 0 && float64(count) >= math.Max(5, mean*2.5) {
			snapshot.Anomalies = append(snapshot.Anomalies, domain.Anomaly{
				Code: "SITE_HAZARD_SURGE", SiteID: siteID,
				Description: "场地隐患数量显著高于同期场地均值", Observed: float64(count),
				Threshold: math.Max(5, mean*2.5), DetectedAt: now,
			})
		}
	}
	if snapshot.Focus.OverdueCount >= 3 && snapshot.Focus.OnTimeRate < 0.6 {
		snapshot.Anomalies = append(snapshot.Anomalies, domain.Anomaly{
			Code: "FOCUS_QUEUE_SLA_DROP", Description: "重点队列超时率异常",
			Observed: snapshot.Focus.OnTimeRate, Threshold: 0.6, DetectedAt: now,
		})
	}
	return snapshot, nil
}

type analyticsWindow struct {
	start time.Time
	end   time.Time
}

func newAnalyticsWindow(from, to time.Time) (analyticsWindow, error) {
	if from.IsZero() || to.IsZero() {
		return analyticsWindow{}, domain.NewValidationError("period", "统计区间不能为空")
	}
	window := analyticsWindow{start: from.UTC(), end: to.UTC()}
	if !window.start.Before(window.end) {
		return analyticsWindow{}, domain.NewValidationError("period", "统计开始时间必须早于结束时间")
	}
	if window.duration() > 366*24*time.Hour {
		return analyticsWindow{}, domain.NewValidationError("period", "统计区间不能超过一年")
	}
	return window, nil
}

func (w analyticsWindow) duration() time.Duration {
	return w.end.Sub(w.start)
}

func (w analyticsWindow) repositoryEnd() time.Time {
	return w.end.Add(time.Nanosecond)
}

func (w analyticsWindow) contains(value time.Time) bool {
	value = value.UTC()
	if value.Before(w.start) {
		return false
	}
	if value.After(w.end) {
		return false
	}
	return true
}

func (w analyticsWindow) split(hazards []domain.Hazard) (normal, focus []domain.Hazard) {
	normal = make([]domain.Hazard, 0, len(hazards))
	focus = make([]domain.Hazard, 0, len(hazards))
	for _, hazard := range hazards {
		if !w.contains(hazard.CreatedAt) {
			continue
		}
		if hazard.Queue == domain.QueueFocus {
			focus = append(focus, hazard)
		} else {
			normal = append(normal, hazard)
		}
	}
	return normal, focus
}

func calculateQueueMetrics(queue domain.QueueKind, hazards []domain.Hazard, now time.Time) domain.QueueMetrics {
	metrics := domain.QueueMetrics{Queue: queue}
	handleMinutes := 0.0
	for _, hazard := range hazards {
		if hazard.State != domain.StateClosed {
			metrics.OpenCount++
			if !hazard.DueAt.IsZero() && now.After(hazard.DueAt) {
				metrics.OverdueCount++
			}
		}
		if hazard.State == domain.StateResolved || hazard.State == domain.StateClosed {
			metrics.ResolvedTotal++
			completedAt := hazard.UpdatedAt
			handleMinutes += completedAt.Sub(hazard.CreatedAt).Minutes()
			if hazard.DueAt.IsZero() || !completedAt.After(hazard.DueAt) {
				metrics.ResolvedOnTime++
			}
		}
	}
	if metrics.ResolvedTotal > 0 {
		metrics.AverageHandleMins = math.Round((handleMinutes/float64(metrics.ResolvedTotal))*100) / 100
		metrics.OnTimeRate = math.Round((float64(metrics.ResolvedOnTime)/float64(metrics.ResolvedTotal))*10000) / 10000
	}
	return metrics
}

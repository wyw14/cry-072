package platform

import (
	"sync"
	"time"
)

type TimeSource interface {
	Now() time.Time
}

type UTCSource struct{}

func NewUTCSource() UTCSource { return UTCSource{} }

func (UTCSource) Now() time.Time { return time.Now().UTC() }

type ManualTimeSource struct {
	mu      sync.RWMutex
	current time.Time
}

func NewManualTimeSource(current time.Time) *ManualTimeSource {
	return &ManualTimeSource{current: current.UTC()}
}

func (s *ManualTimeSource) Now() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *ManualTimeSource) Advance(duration time.Duration) time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = s.current.Add(duration)
	return s.current
}

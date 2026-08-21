package domain

import "strings"

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

var riskRanks = map[RiskLevel]int{
	RiskLow: 1, RiskMedium: 2, RiskHigh: 3, RiskCritical: 4,
}

func (r RiskLevel) Valid() bool {
	_, known := riskRanks[r]
	return known
}

func (r RiskLevel) Rank() int {
	return riskRanks[r]
}

type HazardState string

const (
	StatePending  HazardState = "pending_confirmation"
	StateHandling HazardState = "handling"
	StateReview   HazardState = "pending_review"
	StateResolved HazardState = "resolved"
	StateClosed   HazardState = "closed"
)

var recognizedHazardStates = map[HazardState]struct{}{
	StatePending: {}, StateHandling: {}, StateReview: {}, StateResolved: {}, StateClosed: {},
}

func (s HazardState) Valid() bool {
	_, known := recognizedHazardStates[s]
	return known
}

type QueueKind string

const (
	QueueNormal QueueKind = "normal"
	QueueFocus  QueueKind = "focus"
)

func (q QueueKind) Valid() bool { return q == QueueNormal || q == QueueFocus }

type SiteState string

const (
	SiteActive   SiteState = "active"
	SiteIsolated SiteState = "isolated"
)

func (s SiteState) Valid() bool { return s == SiteActive || s == SiteIsolated }

type Role string

const (
	RoleInspector  Role = "inspector"
	RoleSupervisor Role = "supervisor"
	RoleReviewer   Role = "reviewer"
	RoleAdmin      Role = "admin"
)

type roleCapability uint8

const (
	capabilityConfigureRules roleCapability = 1 << iota
	capabilityReview
	capabilityDowngrade
)

var roleCapabilities = map[Role]roleCapability{
	RoleInspector:  0,
	RoleSupervisor: capabilityConfigureRules | capabilityDowngrade,
	RoleReviewer:   capabilityReview,
	RoleAdmin:      capabilityConfigureRules | capabilityReview | capabilityDowngrade,
}

func ParseRole(value string) Role { return Role(strings.ToLower(strings.TrimSpace(value))) }

func (r Role) Valid() bool {
	_, known := roleCapabilities[r]
	return known
}

func (r Role) permits(capability roleCapability) bool {
	return roleCapabilities[r]&capability != 0
}

func (r Role) CanConfigureRules() bool { return r.permits(capabilityConfigureRules) }

func (r Role) CanReview() bool { return r.permits(capabilityReview) }

func (r Role) CanDowngrade() bool { return r.permits(capabilityDowngrade) }

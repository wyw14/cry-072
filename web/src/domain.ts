export type RiskLevel = 'low' | 'medium' | 'high' | 'critical'
export type HazardState = 'pending_confirmation' | 'handling' | 'pending_review' | 'resolved' | 'closed'
export type QueueKind = 'normal' | 'focus'

export interface Hazard {
  id: string
  site_id: string
  facility_id: string
  risk_category_id: string
  title: string
  description: string
  risk_score: number
  risk_level: RiskLevel
  impact_scopes: string[]
  initial_action: string
  state: HazardState
  queue: QueueKind
  escalated: boolean
  escalation_reason: string
  matched_rule_id: string
  matched_rule_version: number
  matched_rule_evidence: string[]
  owner_id: string
  due_at: string
  version: number
  created_at: string
  updated_at: string
}

export interface Page<T> {
  items: T[]
  page: number
  page_size: number
  total: number
  total_pages: number
}

export interface RuleCondition {
  minimum_score: number
  minimum_level: RiskLevel | ''
  risk_category_ids: string[]
  required_impact_tags: string[]
  minimum_evidence: number
  critical_facility?: boolean
}

export interface EscalationRule {
  id: string
  name: string
  description: string
  priority: number
  enabled: boolean
  version: number
  condition: RuleCondition
  focus_sla: number
  notify_after: number
  renotify_every: number
}

export interface QueueMetrics {
  queue: QueueKind
  open_count: number
  overdue_count: number
  average_handle_minutes: number
  resolved_on_time: number
  resolved_total: number
  on_time_rate: number
}

export interface AnalyticsSnapshot {
  from: string
  to: string
  normal: QueueMetrics
  focus: QueueMetrics
  risk_distribution: Array<{ level: RiskLevel; count: number }>
  anomalies: Array<{ code: string; site_id: string; description: string; observed: number; threshold: number }>
  generated_at: string
}

export interface AuditEvent {
  id: string
  event_type: string
  actor_id: string
  actor_role: string
  rule_id: string
  rule_version: number
  previous_state: HazardState | ''
  current_state: HazardState | ''
  reason: string
  details: Record<string, unknown>
  occurred_at: string
}

export interface APIError {
  code: string
  message: string
  field_errors?: Array<{ field: string; message: string }>
  request_id: string
}
